package main

import (
	"fmt"
	"os"
	"sync"
)

var (
	fileMu      sync.Mutex
	fileHandles = make(map[string]*os.File)
)

func writeToFile(filename, line string) {
	fileMu.Lock()
	defer fileMu.Unlock()

	f, ok := fileHandles[filename]
	if !ok {
		var err error
		f, err = os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Printf("WARNING: Error reading %s: %v\n", filename, err)
			return
		}
		fileHandles[filename] = f
	}

	fmt.Fprintln(f, line)
}

func closeAllFiles() {
	fileMu.Lock()
	defer fileMu.Unlock()
	for name, f := range fileHandles {
		if err := f.Close(); err != nil {
			fmt.Printf("WARNING: Failed to close %s: %v\n", name, err)
		}
		delete(fileHandles, name)
	}
}

func criticalWrite(filename, line string, maxAttempts int) {
	for i := 0; i < maxAttempts; i++ {
		fileMu.Lock()
		f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			fileMu.Unlock()
			continue
		}
		_, writeErr := fmt.Fprintln(f, line)
		f.Close()
		fileMu.Unlock()
		if writeErr == nil {
			return
		}
	}
	fmt.Printf("CRITICAL: Failed to write to %s after %d attempts\n", filename, maxAttempts)
}

const (
	FileAllHits        = "all_hits.txt"
	FileMSValid        = "ms_valid.txt"
	FileHypixelBan     = "hypixel_ban.txt"
	FileHypixelUnban   = "hypixel_unban.txt"
	FileHypixelStats   = "hypixel_stats.txt"
	FileDonutOnline    = "donut_unban_online.txt"
	FileDonutOffline   = "donut_unban_offline.txt"
	FileDonutStats     = "donut_stats.txt"
	FileNitroValid     = "nitro_valid_codes.txt"
	FileNitroPromo     = "nitro_promo_links.txt"
	FileXboxValid      = "valid_xbox_codes.txt"
	FileRewardHits     = "reward_point_hits.txt"
	FileRewardSorted   = "reward_point_hits_sorted.txt"
	FileMCCapture      = "mc_capture.txt"
	FileMSBalance      = "ms_balance_hits.txt"
	FileSetName        = "set_name.txt"
	FileRMTxt          = "RM.txt"
	FileBanUnknown     = "ban_check_unknown_errors.txt"
)

func printSummary() {
	fmt.Printf("\n=================================\n")
	fmt.Printf("         MCChecker Results\n")
	fmt.Printf("=================================\n")
	fmt.Printf("Total Combo    : %d\n", totalChecked)
	fmt.Printf("Valid          : %d\n", validCount)
	fmt.Printf("Invalid        : %d\n", invalidCount)
	fmt.Printf("Locked         : %d\n", lockedCount)
	fmt.Printf("MC Hits        : %d\n", mcHits)
	fmt.Printf("[XGPU] Hits    : %d\n", xgpuHits)
	fmt.Printf("RP Hits        : %d\n", rpHits)
	fmt.Printf("Hypixel Banned : %d\n", hypixelBanned)
	fmt.Printf("Hypixel Unban  : %d\n", hypixelUnban)
	fmt.Printf("Donut Unbanned : %d\n", donutUnbanned)
	fmt.Printf("Cookies Total  : %d\n", cookieTotal)
	fmt.Printf("Cookies Valid  : %d\n", cookieValid)
	fmt.Printf("Cookies Invalid: %d\n", cookieInvalid)
	fmt.Printf("=================================\n")
}
