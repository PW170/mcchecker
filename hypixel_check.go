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
	"net/http"
	"strings"
	"time"

	"github.com/Tnze/go-mc/chat"
	mcnet "github.com/Tnze/go-mc/net"
	"github.com/Tnze/go-mc/net/CFB8"
	pk "github.com/Tnze/go-mc/net/packet"
)

const hypixelProtocol = 47

func checkHypixelBan(username, uuidStr, accessToken string) string {
	if username == "Unknown" || uuidStr == "" || accessToken == "" {
		return "hypixel: error (missing account info)"
	}

	hypixelHosts := []string{"mc.hypixel.net:25565", "hypixel.net:25565"}
	for _, host := range hypixelHosts {
		result := tryHypixelConnect(host, username, uuidStr, accessToken)
		if result != "" {
			return result
		}
	}
	return "hypixel: unbanned"
}

func tryHypixelConnect(addr, username, uuidStr, accessToken string) string {
	conn, err := mcnet.DialMCTimeout(addr, 30*time.Second)
	if err != nil {
		if strings.Contains(err.Error(), "dial tcp") || strings.Contains(err.Error(), "i/o timeout") {
			return ""
		}
		return ""
	}
	defer conn.Close()

	err = conn.WritePacket(pk.Marshal(
		0x00,
		pk.VarInt(hypixelProtocol),
		pk.String(addr),
		pk.UnsignedShort(25565),
		pk.VarInt(2),
	))
	if err != nil {
		return ""
	}

	err = conn.WritePacket(pk.Marshal(
		0x00,
		pk.String(username),
	))
	if err != nil {
		return ""
	}

	for {
		var p pk.Packet
		if err := conn.ReadPacket(&p); err != nil {
			return ""
		}

		switch p.ID {
		case 0x00: // Login Disconnect
			var reason chat.Message
			if err := p.Scan(&reason); err != nil {
				return ""
			}
			reasonStr := reason.ClearString()
			reasonLower := strings.ToLower(reasonStr)

			switch {
			case strings.Contains(reasonStr, "is currently closed"),
				strings.Contains(reasonStr, "Failed cloning"):
				return ""

			case strings.Contains(reasonLower, "you are banned"),
				strings.Contains(reasonLower, "permanently banned"),
				strings.Contains(reasonStr, "You are permanently banned"):
				return "hypixel: banned (permanent)"
			case strings.Contains(reasonLower, "temporarily banned"),
				strings.Contains(reasonLower, "temporary ban"):
				return "hypixel: banned (temp)"
			case strings.Contains(reasonStr, "Suspicious activity"):
				return "hypixel: banned (permanent - suspicious activity)"
			case strings.Contains(reasonLower, "security ban"):
				return "hypixel: banned (security)"

			case strings.Contains(reasonLower, "cannot be used to play on hypixel"),
				strings.Contains(reasonLower, "unsupported"),
				strings.Contains(reasonLower, "version"):
				return ""

			default:
				return fmt.Sprintf("hypixel: banned (%s)", truncateStr(reasonStr, 80))
			}

		case 0x01: // Encryption Request
			var er encryptionReq
			if err := p.Scan(&er); err != nil {
				return ""
			}
			if err := handleHypixelEncryption(conn, accessToken, username, uuidStr, er); err != nil {
				return ""
			}

		case 0x02: // Login Success
			return "hypixel: unbanned"

		case 0x03: // Set Compression
			var threshold pk.VarInt
			if err := p.Scan(&threshold); err != nil {
				return ""
			}
			conn.SetThreshold(int(threshold))

		default:
			if p.ID == 0x02 || p.ID == 0x03 {
				continue
			}
			return "hypixel: unbanned"
		}
	}
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
