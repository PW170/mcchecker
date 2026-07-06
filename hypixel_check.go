package main

import (
	"bytes"
	"crypto/aes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Tnze/go-mc/chat"
	mcnet "github.com/Tnze/go-mc/net"
	"github.com/Tnze/go-mc/net/CFB8"
	pk "github.com/Tnze/go-mc/net/packet"
)

const hypixelProtocol = 47

var hypixelHosts = []string{"mc.hypixel.net:25565", "hypixel.net:25565"}

func checkHypixelBan(username, uuidStr, accessToken string) (string, error) {
	if username == "Unknown" || uuidStr == "" || accessToken == "" {
		return "", fmt.Errorf("hypixel: missing account info")
	}

	var lastErr error
	for _, host := range hypixelHosts {
		result, err := tryHypixelConnect(host, username, uuidStr, accessToken)
		if err != nil {
			lastErr = err
			continue
		}
		return result, nil
	}

	if lastErr != nil {
		return "", fmt.Errorf("hypixel: all hosts failed: %w", lastErr)
	}
	return "hypixel: unbanned", nil
}

func tryHypixelConnect(addr, username, uuidStr, accessToken string) (string, error) {
	conn, err := mcnet.DialMCTimeout(addr, 30*time.Second)
	if err != nil {
		return "", fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	if err := conn.WritePacket(pk.Marshal(
		0x00,
		pk.VarInt(hypixelProtocol),
		pk.String(addr),
		pk.UnsignedShort(25565),
		pk.VarInt(2),
	)); err != nil {
		return "", fmt.Errorf("handshake: %w", err)
	}

	if err := conn.WritePacket(pk.Marshal(
		0x00,
		pk.String(username),
	)); err != nil {
		return "", fmt.Errorf("login start: %w", err)
	}

	loggedIn := false
	playDeadline := time.Now().Add(20 * time.Second)

	for {
		if loggedIn && time.Now().After(playDeadline) {
			return "hypixel: unbanned", nil
		}

		if loggedIn {
			conn.Socket.SetReadDeadline(time.Now().Add(10 * time.Second))
		}

		var p pk.Packet
		if err := conn.ReadPacket(&p); err != nil {
			if loggedIn {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					return "hypixel: unbanned", nil
				}
				return "", fmt.Errorf("play read: %w", err)
			}
			return "", fmt.Errorf("read packet: %w", err)
		}

		if loggedIn {
			switch p.ID {
			case 0x01:
				return "hypixel: unbanned", nil
			case 0x40:
				var reason chat.Message
				if err := p.Scan(&reason); err != nil {
					return "hypixel: unbanned", nil
				}
				return parseBanReason(reason), nil
			case 0x00:
				continue
			default:
				continue
			}
		}

		switch p.ID {
		case 0x00:
			var reason chat.Message
			if err := p.Scan(&reason); err != nil {
				return "", fmt.Errorf("parse disconnect: %w", err)
			}
			return parseBanReason(reason), nil

		case 0x01:
			var er encryptionReq
			if err := p.Scan(&er); err != nil {
				return "", fmt.Errorf("parse encryption: %w", err)
			}
			if err := handleHypixelEncryption(conn, accessToken, username, uuidStr, er); err != nil {
				return "", fmt.Errorf("encryption: %w", err)
			}

		case 0x02:
			loggedIn = true

		case 0x03:
			var threshold pk.VarInt
			if err := p.Scan(&threshold); err != nil {
				return "", fmt.Errorf("parse compression: %w", err)
			}
			conn.SetThreshold(int(threshold))

		default:
			return "", fmt.Errorf("unexpected packet id 0x%02x", p.ID)
		}
	}
}

func parseBanReason(msg chat.Message) string {
	text := msg.ClearString()
	text = strings.ReplaceAll(text, "\n", " | ")
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	lower := strings.ToLower(text)

	banType := ""
	banDur := ""
	banReason := ""

	if strings.Contains(lower, "permanently banned") || strings.Contains(lower, "permanent ban") {
		banType = "permanent"
	} else if strings.Contains(lower, "temporarily banned") || strings.Contains(lower, "temporary ban") {
		banType = "temporary"
	} else if strings.Contains(lower, "security ban") {
		banType = "security"
	} else if strings.Contains(lower, "suspicious") {
		banType = "permanent"
		banReason = "Suspicious activity"
	} else {
		banType = "temporary"
	}

	if banDur == "" {
		durRe := regexp.MustCompile(`(?i)(?:for|banned for)\s+(\d+)\s*(day|days|hour|hours|minute|minutes|month|months|year|years)`)
		if m := durRe.FindStringSubmatch(text); len(m) >= 3 {
			banDur = m[1] + " " + m[2]
		}
	}
	if banDur == "" && banType == "permanent" {
		banDur = "permanent"
	}

	if banReason == "" {
		reasonRe := regexp.MustCompile(`(?i)reason:\s*(.+?)(?:\.|\||$)`)
		if m := reasonRe.FindStringSubmatch(text); len(m) >= 2 {
			banReason = strings.TrimSpace(m[1])
		}
	}

	result := "hypixel: banned"
	if banType != "" {
		result += " (" + banType
	}
	if banDur != "" {
		if banType != "" {
			result += " - " + banDur
		} else {
			result += " (" + banDur
		}
	}
	if banReason != "" {
		result += " - " + banReason
	}
	if banType != "" || banDur != "" || banReason != "" {
		result += ")"
	}
	return result
}

type encryptionReq struct {
	ServerID    string
	PublicKey   []byte
	VerifyToken []byte
}

func (e *encryptionReq) ReadFrom(r io.Reader) (int64, error) {
	return pk.Tuple{
		(*pk.String)(&e.ServerID),
		(*pk.ByteArray)(&e.PublicKey),
		(*pk.ByteArray)(&e.VerifyToken),
	}.ReadFrom(r)
}

func handleHypixelEncryption(conn *mcnet.Conn, accessToken, username, uuidStr string, er encryptionReq) error {
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		return err
	}

	digest := hypixelAuthDigest(er.ServerID, key, er.PublicKey)
	if err := hypixelMojangAuth(accessToken, username, uuidStr, digest); err != nil {
		return err
	}

	iPK, err := x509.ParsePKIXPublicKey(er.PublicKey)
	if err != nil {
		return err
	}
	rsaKey := iPK.(*rsa.PublicKey)

	cryptKey, err := rsa.EncryptPKCS1v15(rand.Reader, rsaKey, key)
	if err != nil {
		return err
	}
	cryptToken, err := rsa.EncryptPKCS1v15(rand.Reader, rsaKey, er.VerifyToken)
	if err != nil {
		return err
	}

	if err := conn.WritePacket(pk.Marshal(
		0x01,
		pk.ByteArray(cryptKey),
		pk.ByteArray(cryptToken),
	)); err != nil {
		return err
	}

	b, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	conn.SetCipher(CFB8.NewCFB8Encrypt(b, key), CFB8.NewCFB8Decrypt(b, key))
	return nil
}

func hypixelAuthDigest(serverID string, sharedSecret, publicKey []byte) string {
	h := sha1.New()
	h.Write([]byte(serverID))
	h.Write(sharedSecret)
	h.Write(publicKey)
	hash := h.Sum(nil)

	negative := (hash[0] & 0x80) == 0x80
	if negative {
		hash = twosComplement(hash)
	}

	res := strings.TrimLeft(hex.EncodeToString(hash), "0")
	if negative {
		res = "-" + res
	}
	return res
}

func twosComplement(p []byte) []byte {
	carry := true
	for i := len(p) - 1; i >= 0; i-- {
		p[i] = ^p[i]
		if carry {
			carry = p[i] == 0xff
			p[i]++
		}
	}
	return p
}

func hypixelMojangAuth(accessToken, username, uuidStr, serverHash string) error {
	payload := map[string]string{
		"accessToken":     accessToken,
		"selectedProfile": uuidStr,
		"serverId":        serverHash,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "https://sessionserver.mojang.com/session/minecraft/join",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("mojang auth failed: %s", string(respBody))
	}
	return nil
}
