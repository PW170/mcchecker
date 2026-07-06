package main

import "fmt"

type LogLevel int

const (
	LogInfo    LogLevel = iota
	LogSuccess
	LogError
	LogHypixel
	LogProgress
)

type ProgressStats struct {
	TotalChecked   int64   `json:"totalChecked"`
	MCHits         int64   `json:"mcHits"`
	XGPUHits       int64   `json:"xgpuHits"`
	RPHits         int64   `json:"rpHits"`
	ValidCount     int64   `json:"validCount"`
	HypixelBanned  int64   `json:"hypixelBanned"`
	HypixelUnban   int64   `json:"hypixelUnban"`
	CookieValid    int64   `json:"cookieValid"`
	CookieInvalid  int64   `json:"cookieInvalid"`
	CookieTotal    int64   `json:"cookieTotal"`
	ElapsedSeconds float64 `json:"elapsedSeconds"`
	CPM            float64 `json:"cpm"`
}

type CheckSummary struct {
	ProgressStats
	TotalCombos int `json:"totalCombos"`
	TotalCookies int `json:"totalCookies"`
	RunDir       string `json:"runDir"`
}

var OnLog func(level LogLevel, msg string)

var OnProgress func(stats ProgressStats)

var OnComplete func(summary CheckSummary)

var IsTerminal bool

func logLine(format string, args ...interface{}) {
	if OnLog != nil {
		OnLog(LogInfo, fmt.Sprintf(format, args...))
	}
}

func logSuccess(format string, args ...interface{}) {
	if OnLog != nil {
		OnLog(LogSuccess, fmt.Sprintf(format, args...))
	}
}

func logErrorLine(format string, args ...interface{}) {
	if OnLog != nil {
		OnLog(LogError, fmt.Sprintf(format, args...))
	}
}

func logHypixel(format string, args ...interface{}) {
	if OnLog != nil {
		OnLog(LogHypixel, fmt.Sprintf(format, args...))
	}
}
