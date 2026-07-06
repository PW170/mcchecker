package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

const (
	Version    = "MCChecker/1.0"
	APIBaseURL = "https://api.mcchecker.local"
	UserAgent  = "MCChecker-Go/1.0"
)

var (
	totalChecked   int64
	validCount     int64
	invalidCount   int64
	lockedCount    int64
	mcHits         int64
	xgpuHits       int64
	rpHits         int64
	hypixelBanned  int64
	hypixelUnban   int64
	cookieTotal    int64
	cookieValid    int64
	cookieInvalid  int64
	currentRunDir  string
	proxyIndex     int64
	checkingStopped int32
)

type cookieXboxResult struct {
	client     *http.Client
	cookieFile string
	rawLines   []string
	uhs        string
	bearer     string
}

func loadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		def := defaultConfig()
		data, _ := json.MarshalIndent(def, "", "  ")
		os.WriteFile(path, data, 0644)
		return def, nil
	}
	defer f.Close()
	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		def := defaultConfig()
		data, _ := json.MarshalIndent(def, "", "  ")
		os.WriteFile(path, data, 0644)
		return def, nil
	}
	return &cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		CookieCheck:      true,
		CookiePath:       "cookies",
		BanCheck:         true,
		HypixelCheck:     true,
		IncludeHypixel:   true,
		MSRewards:        true,
		XboxPerks:        true,
		GamepassPC:       true,
		GamepassUltimate: true,
		HypixelBan:       true,
		HypixelUnban:    true,
		HypixelRanked:   true,
		RateLimiting:    true,
		RetryRateLimited: true,
		ShowHits:        true,
	}
}

func loadFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}

func removeDuplicates(lines []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, l := range lines {
		if !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	return out
}

func pickProxy(proxies []string, mode string) string {
	if len(proxies) == 0 || mode == "" {
		return ""
	}
	idx := atomic.AddInt64(&proxyIndex, 1) - 1
	return proxies[idx%int64(len(proxies))]
}

func getNextRunDir() string {
	idx := 1
	for {
		name := filepath.Join("results", fmt.Sprintf("R%d", idx))
		if _, err := os.Stat(name); os.IsNotExist(err) {
			return name
		}
		idx++
	}
}

func runSetup() {
	os.MkdirAll("cookies", 0755)
	os.MkdirAll("results", 0755)
	os.MkdirAll("errors", 0755)
	if _, err := os.Stat("config.json"); os.IsNotExist(err) {
		def := defaultConfig()
		data, _ := json.MarshalIndent(def, "", "  ")
		os.WriteFile("config.json", data, 0644)
	}
	placeholders := map[string]string{
		"combos.txt":  "; Paste your email:password combos here, one per line",
		"proxies.txt": "; Paste your proxies here, one per line (http://user:pass@ip:port)",
	}
	for name, content := range placeholders {
		if _, err := os.Stat(name); os.IsNotExist(err) {
			os.WriteFile(name, []byte(content+"\n"), 0644)
		}
	}
}

func valOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func boolStr(b bool) string {
	if b {
		return "Enabled"
	}
	return "Disabled"
}
