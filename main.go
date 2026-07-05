package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
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
	donutUnbanned  int64
	cookieTotal    int64
	cookieValid    int64
	cookieInvalid  int64
)

func main() {
	console := NewConsole()
	console.Clear()

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
			runChecker(console)
		case "2":
			showConfig(console)
		case "3":
			showCredits(console)
		case "4":
			console.Println(yellow("  Goodbye."))
			return
		default:
			console.Println(red("  Invalid option. Press Enter to continue..."))
			console.ReadLine()
		}
	}
}

func runChecker(console *Console) {
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
			printProgress(startTime)
		}(email, password)
	}

	cookieSem := make(chan struct{}, 5)
	var cookieWg sync.WaitGroup
	for _, cf := range cookieFiles {
		cookieSem <- struct{}{}
		cookieWg.Add(1)
		go func(cf string) {
			defer cookieWg.Done()
			defer func() { <-cookieSem }()
			proxyURL := pickProxy(proxies, cfg.ProxyMode)
			checkCookies(cf, proxyURL, cfg)
			printProgress(startTime)
		}(cf)
		time.Sleep(300 * time.Millisecond)
	}
	cookieWg.Wait()

	wg.Wait()

	closeAllFiles()

	console.Clear()
	printBanner(console)
	console.Println(green("\n  ────────────── CHECK COMPLETE ──────────────\n"))
	console.Println(white(fmt.Sprintf("  Accounts Checked: %d", totalChecked)))
	console.Println(white(fmt.Sprintf("  Valid Accounts:  %d", validCount)))
	console.Println(white(fmt.Sprintf("  Invalid:         %d", invalidCount)))
	console.Println(white(fmt.Sprintf("  Locked:          %d", lockedCount)))
	console.Println(white(fmt.Sprintf("  MC Hits:         %d", mcHits)))
	console.Println(white(fmt.Sprintf("  XGPU Hits:       %d", xgpuHits)))
	console.Println(white(fmt.Sprintf("  RP Hits:         %d", rpHits)))
	console.Println("")
	console.Println(white(fmt.Sprintf("  Cookies Checked: %d", cookieTotal)))
	console.Println(white(fmt.Sprintf("  Cookies Valid:   %d", cookieValid)))
	console.Println(white(fmt.Sprintf("  Cookies Invalid: %d", cookieInvalid)))
	console.Println("")
	console.Println(white(fmt.Sprintf("  Time Elapsed:    %s", time.Since(startTime).Round(time.Second))))
	console.Println(green("\n  ────────────────────────────────────────────"))
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
	console.Println(white(fmt.Sprintf("  Cookie Check:    %s", boolStr(cfg.CookieCheck))))
	console.Println(white(fmt.Sprintf("  Cookie Path:     %s", valOr(cfg.CookiePath, "cookies/"))))
	console.Println(white(fmt.Sprintf("  Hypixel Check:   %s", boolStr(cfg.HypixelCheck))))
	console.Println(white(fmt.Sprintf("  Donut Check:     %s", boolStr(cfg.DonutCheck))))
	console.Println(white(fmt.Sprintf("  MS Rewards:      %s", boolStr(cfg.MSRewards))))
	console.Println(white(fmt.Sprintf("  Xbox Perks:      %s", boolStr(cfg.XboxPerks))))
	console.Println(white(fmt.Sprintf("  Nitro Promo:     %s", boolStr(cfg.NitroPromo))))
	console.Println(white(fmt.Sprintf("  Proxy Mode:      %s", valOr(cfg.ProxyMode, "none"))))
	console.Println(white(fmt.Sprintf("  Retry on Rate:   %s", boolStr(cfg.RetryRateLimited))))
	console.Println(white(fmt.Sprintf("  Sniper:          %s", boolStr(cfg.Sniper))))
	console.Println(white(fmt.Sprintf("  Gamepass PC:     %s", boolStr(cfg.GamepassPC))))
	console.Println(white(fmt.Sprintf("  Gamepass Ult:    %s", boolStr(cfg.GamepassUltimate))))
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

func loadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
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

var proxyIndex int64

func pickProxy(proxies []string, mode string) string {
	if len(proxies) == 0 || mode == "" {
		return ""
	}
	idx := atomic.AddInt64(&proxyIndex, 1) - 1
	return proxies[idx%int64(len(proxies))]
}

func printProgress(start time.Time) {
	checked := atomic.LoadInt64(&totalChecked)
	valid := atomic.LoadInt64(&validCount)
	mc := atomic.LoadInt64(&mcHits)
	xgpu := atomic.LoadInt64(&xgpuHits)
	rp := atomic.LoadInt64(&rpHits)
	cv := atomic.LoadInt64(&cookieValid)
	ci := atomic.LoadInt64(&cookieInvalid)
	elapsed := time.Since(start).Seconds()
	total := checked + atomic.LoadInt64(&cookieTotal)
	cps := float64(total) / elapsed
	fmt.Printf("\r  [MC: %d] [XGPU: %d] [RP: %d] [Val: %d] [Cook: %d/%d] [Total: %d] [%.1f/s]   ",
		mc, xgpu, rp, valid, cv, ci, total, cps)
}

func boolStr(b bool) string {
	if b {
		return green("Enabled")
	}
	return red("Disabled")
}

func valOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func cyan(s string) string  { return "\033[36m" + s + "\033[0m" }
func green(s string) string { return "\033[32m" + s + "\033[0m" }
func yellow(s string) string { return "\033[33m" + s + "\033[0m" }
func red(s string) string   { return "\033[31m" + s + "\033[0m" }
func white(s string) string { return "\033[37m" + s + "\033[0m" }
func gray(s string) string  { return "\033[90m" + s + "\033[0m" }
