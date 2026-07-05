package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	msLoginURL   = "https://login.live.com/oauth20_authorize.srf?client_id=00000000402B5328&redirect_uri=https://login.live.com/oauth20_desktop.srf&scope=service::user.auth.xboxlive.com::MBI_SSL&display=touch&response_type=token&locale=en"
	xboxLiveURL  = "https://user.auth.xboxlive.com"
	xstsURL      = "https://xsts.auth.xboxlive.com/xsts/authorize"
	mcAuthURL    = "https://api.minecraftservices.com/authentication/login_with_xbox"
	mcProfileURL = "https://api.minecraftservices.com/minecraft/profile"
	msAccountURL = "https://account.microsoft.com/"
	gamepassURL  = "https://profile.gamepass.com"
)

type MSAuthTokens struct {
	AccessToken  string
	RefreshToken string
	UAID         string
	PPFT         string
	IPT          string
	PPRid        string
	UrlPost      string
}

type XboxTokenResponse struct {
	Token    string
	UserHash string
}

type MCAuthResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type MCProfile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func fullAuth(email, password, proxyURL string) (*MCAuthResponse, *MCProfile, error) {
	client := buildHTTPClient(proxyURL)

	tokens, err := fetchMSTokens(client, email, password)
	if err != nil {
		return nil, nil, fmt.Errorf("login failed: %w", err)
	}

	xblToken, userHash, err := xboxLiveAuth(client, tokens.AccessToken)
	if err != nil {
		return nil, nil, fmt.Errorf("Xbox auth failed: %w", err)
	}

	xstsToken, err := xstsAuth(client, xblToken)
	if err != nil {
		return nil, nil, fmt.Errorf("XSTS auth failed: %w", err)
	}

	mcToken, err := minecraftAuth(client, xstsToken, userHash)
	if err != nil {
		return nil, nil, fmt.Errorf("Minecraft auth request failed: %w", err)
	}

	profile, err := getMinecraftProfile(client, mcToken.AccessToken)
	if err != nil {
		return mcToken, nil, nil
	}

	return mcToken, profile, nil
}

func fetchMSTokens(client *http.Client, email, password string) (*MSAuthTokens, error) {

	req, _ := http.NewRequest("GET", "https://login.live.com/oauth20_authorize.srf?client_id=00000000402B5328&redirect_uri=https://login.live.com/oauth20_desktop.srf&scope=service::user.auth.xboxlive.com::MBI_SSL&display=touch&response_type=token&locale=en", nil)
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OAuth page: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	ppftRe := regexp.MustCompile(`"PPFT"\s*[^}]*"value"\s*:\s*"([^"]+)"`)
	ppftMatch := ppftRe.FindStringSubmatch(bodyStr)
	if len(ppftMatch) < 2 {

		ppftRe2 := regexp.MustCompile(`sFT\s*[=:]\s*'([^']+)'`)
		ppftMatch = ppftRe2.FindStringSubmatch(bodyStr)
		if len(ppftMatch) < 2 {
			return nil, fmt.Errorf("PPFT token not found in response")
		}
	}
	ppft := ppftMatch[1]

	urlPostRe := regexp.MustCompile(`urlPost:'([^']+)'`)
	urlPostMatch := urlPostRe.FindStringSubmatch(bodyStr)
	if len(urlPostMatch) < 2 {
		urlPostRe2 := regexp.MustCompile(`"urlPost":"(.+?)"`)
		urlPostMatch = urlPostRe2.FindStringSubmatch(bodyStr)
		if len(urlPostMatch) < 2 {
			return nil, fmt.Errorf("URL POST not found in response")
		}
	}
	urlPost := urlPostMatch[1]

	uaidRe := regexp.MustCompile(`"uaid" value="([^"]+)"`)
	uaidMatch := uaidRe.FindStringSubmatch(bodyStr)
	uaid := ""
	if len(uaidMatch) >= 2 {
		uaid = uaidMatch[1]
	}

	data := url.Values{
		"login":       {email},
		"passwd":      {password},
		"PPFT":        {ppft},
		"ps":          {"2"},
		"canary":      {""},
		"type":        {"11"},
		"LoginOptions": {"3"},
	}

	postReq, _ := http.NewRequest("POST", urlPost, strings.NewReader(data.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.Header.Set("User-Agent", UserAgent)
	postReq.Header.Set("Referer", "https://login.live.com/")

	postResp, err := client.Do(postReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth tokens: %w", err)
	}
	defer postResp.Body.Close()

	finalURL := postResp.Request.URL.String()

	tokenRe := regexp.MustCompile(`#access_token=([^&]+)`)
	tokenMatch := tokenRe.FindStringSubmatch(finalURL)
	if len(tokenMatch) < 2 {

		errBody, _ := io.ReadAll(postResp.Body)
		errStr := string(errBody)

		if strings.Contains(errStr, "password is incorrect") || strings.Contains(errStr, "wrong password") {
			return nil, fmt.Errorf("password is incorrect")
		}
		if strings.Contains(errStr, "account doesn't exist") {
			return nil, fmt.Errorf("account doesn't exist")
		}
		if strings.Contains(errStr, "account_locked") || strings.Contains(errStr, "account is locked") {
			return nil, fmt.Errorf("account locked")
		}
		if strings.Contains(errStr, "verification_required") || strings.Contains(errStr, "security verification required") {
			return nil, fmt.Errorf("verification_required")
		}
		if strings.Contains(errStr, "30_day_lockout") {
			return nil, fmt.Errorf("30_day_lockout")
		}
		if strings.Contains(errStr, "rate_limited") {
			return nil, fmt.Errorf("rate_limited")
		}
		if strings.Contains(errStr, "2fa_required") {
			return nil, fmt.Errorf("2fa_required")
		}
		return nil, fmt.Errorf("silent auth: no token returned")
	}

	accessToken := tokenMatch[1]

	return &MSAuthTokens{
		AccessToken: accessToken,
		UAID:        uaid,
		PPFT:        ppft,
		UrlPost:     urlPost,
	}, nil
}

func xboxLiveAuth(client *http.Client, accessToken string) (string, string, error) {
	payload := map[string]interface{}{
		"Properties": map[string]interface{}{
			"AuthMethod": "RPS",
			"SiteName":   "user.auth.xboxlive.com",
			"RpsTicket":  "d=" + accessToken,
		},
		"RelyingParty": "http://auth.xboxlive.com",
		"TokenType":    "JWT",
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "https://user.auth.xboxlive.com/user/authenticate", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("Xbox auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return "", "", fmt.Errorf("Xbox auth rate limited (429)")
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("failed to parse Xbox response: %w", err)
	}

	token, ok := result["Token"].(string)
	if !ok {
		return "", "", fmt.Errorf("Xbox auth failed: %w", fmt.Errorf("no UHS in XSTS response"))
	}

	displayClaims, _ := result["DisplayClaims"].(map[string]interface{})
	xui, _ := displayClaims["xui"].([]interface{})
	userHash := ""
	if len(xui) > 0 {
		xuiMap, _ := xui[0].(map[string]interface{})
		userHash, _ = xuiMap["uhs"].(string)
	}

	return token, userHash, nil
}

func xstsAuth(client *http.Client, xblToken string) (string, error) {
	payload := map[string]interface{}{
		"Properties": map[string]interface{}{
			"SandboxId":  "RETAIL",
			"UserTokens": []string{xblToken},
		},
		"RelyingParty": "rp://api.minecraftservices.com/",
		"TokenType":    "JWT",
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", xstsURL, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("XSTS auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return "", fmt.Errorf("XSTS auth rate limited (429)")
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse XSTS response: %w", err)
	}

	token, ok := result["Token"].(string)
	if !ok {
		return "", fmt.Errorf("XSTS auth failed: no token")
	}

	return token, nil
}

func minecraftAuth(client *http.Client, xstsToken, userHash string) (*MCAuthResponse, error) {
	identityToken := fmt.Sprintf("XBL3.0 x=%s;%s", userHash, xstsToken)
	payload := map[string]string{
		"identityToken": identityToken,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", mcAuthURL, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Minecraft auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("Minecraft auth rate limited (429)")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Minecraft auth failed (status %d)", resp.StatusCode)
	}

	var result MCAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func getMinecraftProfile(client *http.Client, accessToken string) (*MCProfile, error) {
	req, _ := http.NewRequest("GET", mcProfileURL, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("profile not found (status %d)", resp.StatusCode)
	}

	var profile MCProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

func buildHTTPClient(proxyURL string) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:       100,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: false,
	}

	if proxyURL != "" {
		pURL, err := url.Parse(proxyURL)
		if err == nil {
			transport.Proxy = http.ProxyURL(pURL)
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}
}
