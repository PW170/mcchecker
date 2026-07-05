package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	mathrand "math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func parseCookieFile(path string) (map[string]string, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot open cookie file %s: %w", path, err)
	}
	defer f.Close()

	cookies := make(map[string]string)
	var rawLines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		rawLines = append(rawLines, line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) >= 7 {
			domain := fields[0]
			if strings.Contains(domain, "login.live.com") {
				cookies[fields[5]] = fields[6]
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return cookies, rawLines, fmt.Errorf("error reading cookie file: %w", err)
	}
	return cookies, rawLines, nil
}

func uuidV4() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func randomDigitStr(length int) string {
	digits := "012346789"
	result := make([]byte, length)
	for i := range result {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		result[i] = digits[idx.Int64()]
	}
	return string(result)
}

func decodeXboxFragment(fragment string) (string, string, error) {
	parsed, _ := url.ParseQuery(fragment)
	tokenB64 := parsed.Get("accessToken")
	if tokenB64 == "" {
		return "", "", fmt.Errorf("no access token found in redirect fragment")
	}

	switch len(tokenB64) % 4 {
	case 2:
		tokenB64 += "=="
	case 3:
		tokenB64 += "="
	}

	decoded, err := base64.StdEncoding.DecodeString(tokenB64)
	if err != nil {
		return "", "", fmt.Errorf("base64 decode of xbox token failed: %w", err)
	}

	var items []map[string]interface{}
	if err := json.Unmarshal(decoded, &items); err != nil {
		return "", "", fmt.Errorf("json parse of xbox token failed: %w", err)
	}

	var token, uhs string
	for _, item := range items {
		item1, _ := item["Item1"].(string)
		if item1 == "rp://api.minecraftservices.com/" {
			item2, _ := item["Item2"].(map[string]interface{})
			if item2 != nil {
				token, _ = item2["Token"].(string)
				dc, _ := item2["DisplayClaims"].(map[string]interface{})
				if dc != nil {
					xui, _ := dc["xui"].([]interface{})
					if len(xui) > 0 {
						xuiMap, _ := xui[0].(map[string]interface{})
						uhs, _ = xuiMap["uhs"].(string)
					}
				}
			}
		}
	}

	if token == "" || uhs == "" {
		return "", "", fmt.Errorf("failed to extract xbox token or user hash from auth response")
	}

	return uhs, token, nil
}

func buildNoRedirectClient() *http.Client {
	return &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return http.ErrUseLastResponse
		},
	}
}

func cookieXboxAuth(client *http.Client, msauthCookie string) (uhs, bearer string, err error) {
	noRedirect := buildNoRedirectClient()
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:122.0) Gecko/20100101 Firefox/122.0"

	xboxURL := fmt.Sprintf("https://sisu.xboxlive.com/connect/XboxLive/?state=login&cobrandId=%s&tid=%s&ru=https://www.minecraft.net/en-us/login&aid=%s",
		uuidV4(), randomDigitStr(9), randomDigitStr(10))

	req1, _ := http.NewRequest("GET", xboxURL, nil)
	req1.Header.Set("User-Agent", ua)
	req1.Header.Set("Referer", "https://www.minecraft.net/")
	req1.AddCookie(&http.Cookie{Name: "__Host-MSAAUTHP", Value: msauthCookie})

	resp1, err := noRedirect.Do(req1)
	if err != nil {
		return "", "", fmt.Errorf("xbox sisu failed: %w", wrapNetError(err))
	}

	if resp1.StatusCode != 302 && resp1.StatusCode != 301 {
		body, _ := io.ReadAll(resp1.Body)
		resp1.Body.Close()
		return "", "", fmt.Errorf("no redirect from xbox sisu (HTTP %d): %s", resp1.StatusCode, string(body[:min(len(body), 200)]))
	}

	sisuLoc := resp1.Header.Get("Location")
	resp1.Body.Close()
	if sisuLoc == "" {
		return "", "", fmt.Errorf("xbox sisu returned %d but Location header is empty", resp1.StatusCode)
	}

	parsedURL, _ := url.Parse(sisuLoc)
	q := parsedURL.Query()
	q.Del("nopa")
	parsedURL.RawQuery = q.Encode()
	oauthURL := parsedURL.String()

	req2, _ := http.NewRequest("GET", oauthURL, nil)
	req2.Header.Set("User-Agent", ua)
	req2.AddCookie(&http.Cookie{Name: "__Host-MSAAUTHP", Value: msauthCookie})

	resp2, err := noRedirect.Do(req2)
	if err != nil {
		return "", "", fmt.Errorf("login.live.com oauth failed: %w", wrapNetError(err))
	}

	if resp2.StatusCode != 302 && resp2.StatusCode != 301 {
		body, _ := io.ReadAll(resp2.Body)
		resp2.Body.Close()
		return "", "", fmt.Errorf("login.live.com oauth rejected cookie (HTTP %d): %s", resp2.StatusCode, string(body[:min(len(body), 200)]))
	}

	codeLoc := resp2.Header.Get("Location")
	resp2.Body.Close()
	if codeLoc == "" {
		return "", "", fmt.Errorf("login.live.com returned %d but no Location header", resp2.StatusCode)
	}

	currentURL := codeLoc
	for i := 0; i < 10; i++ {
		parsed, _ := url.Parse(currentURL)
		if parsed != nil && parsed.Fragment != "" {
			uhs, bearer, err := decodeXboxFragment(parsed.Fragment)
			if err != nil {
				return "", "", fmt.Errorf("failed to decode xbox auth fragment: %w", err)
			}
			return uhs, bearer, nil
		}

		req, _ := http.NewRequest("GET", currentURL, nil)
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Referer", "https://www.minecraft.net/")

		resp, err := noRedirect.Do(req)
		if err != nil {
			return "", "", fmt.Errorf("redirect chain failed at step %d: %w", i+1, wrapNetError(err))
		}

		if resp.StatusCode != 302 && resp.StatusCode != 301 {
			resp.Body.Close()
			return "", "", fmt.Errorf("redirect chain ended at step %d (HTTP %d) without token", i+1, resp.StatusCode)
		}

		currentURL = resp.Header.Get("Location")
		resp.Body.Close()
		if currentURL == "" {
			return "", "", fmt.Errorf("redirect chain broken at step %d — empty Location", i+1)
		}
	}

	return "", "", fmt.Errorf("redirect chain exceeded maximum depth (10)")
}

func cookieMCAuth(client *http.Client, uhs, bearer string) (*MCAuthResponse, *MCProfile, error) {
	identityToken := fmt.Sprintf("XBL3.0 x=%s;%s", uhs, bearer)
	payload := map[string]string{"identityToken": identityToken}
	body, _ := json.Marshal(payload)

	var mcResp MCAuthResponse
	for retry := 0; retry < 5; retry++ {
		if retry > 0 {
			backoff := time.Duration(1<<uint(retry)) * time.Second
			jitter := time.Duration(mathrand.Int63n(int64(backoff) / 2))
			time.Sleep(backoff + jitter)
		}

		mcAuthRateLimiter.Wait()

		mcReq, _ := http.NewRequest("POST", mcAuthURL, strings.NewReader(string(body)))
		mcReq.Header.Set("Content-Type", "application/json")
		mcReq.Header.Set("Accept", "application/json")
		mcReq.Header.Set("User-Agent", UserAgent)

		mcRespRaw, mcErr := client.Do(mcReq)
		if mcErr != nil {
			return nil, nil, fmt.Errorf("minecraft auth request failed: %w", wrapNetError(mcErr))
		}

		if mcRespRaw.StatusCode == 429 {
			mcRespRaw.Body.Close()
			if retry >= 4 {
				return nil, nil, fmt.Errorf("mc auth failed (status 429) — rate limited after 5 retries")
			}
			continue
		}
		if mcRespRaw.StatusCode == 401 {
			mcRespRaw.Body.Close()
			return nil, nil, fmt.Errorf("mc auth failed (status 401) — xbox token rejected")
		}
		if mcRespRaw.StatusCode != 200 {
			b, _ := io.ReadAll(mcRespRaw.Body)
			mcRespRaw.Body.Close()
			return nil, nil, fmt.Errorf("mc auth failed (status %d): %s", mcRespRaw.StatusCode, string(b[:min(len(b), 200)]))
		}

		if err := json.NewDecoder(mcRespRaw.Body).Decode(&mcResp); err != nil {
			mcRespRaw.Body.Close()
			return nil, nil, fmt.Errorf("mc auth response parse failed: %w", err)
		}
		mcRespRaw.Body.Close()

		if mcResp.AccessToken == "" {
			return nil, nil, fmt.Errorf("mc auth returned empty access token")
		}

		profile, err := getMinecraftProfile(client, mcResp.AccessToken)
		if err != nil {
			return &mcResp, nil, nil
		}
		return &mcResp, profile, nil
	}
	return nil, nil, fmt.Errorf("mc auth failed after 5 retries")
}

func cookieFullAuth(client *http.Client, msauthCookie string) (*MCAuthResponse, *MCProfile, error) {
	uhs, bearer, err := cookieXboxAuth(client, msauthCookie)
	if err != nil {
		return nil, nil, err
	}
	return cookieMCAuth(client, uhs, bearer)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
