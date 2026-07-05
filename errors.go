package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type CheckError struct {
	Category string
	Message  string
	Detail   string
}

func (e *CheckError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Category, e.Message)
}

func categorizeCookieError(err error) *CheckError {
	if err == nil {
		return nil
	}
	errStr := err.Error()

	switch {
	case strings.Contains(errStr, "no redirect from outlook"),
		strings.Contains(errStr, "no auth cookie in response"),
		strings.Contains(errStr, "expired"):
		return &CheckError{Category: "EXPIRED", Message: "Cookie expired or invalid", Detail: errStr}

	case strings.Contains(errStr, "no __Host-MSAAUTHP"):
		return &CheckError{Category: "MISSING", Message: "__Host-MSAAUTHP cookie not found", Detail: errStr}

	case strings.Contains(errStr, "no access token"),
		strings.Contains(errStr, "base64 decode"),
		strings.Contains(errStr, "json decode"),
		strings.Contains(errStr, "failed to extract"):
		return &CheckError{Category: "TOKEN_ERROR", Message: "Failed to parse auth token", Detail: errStr}

	case strings.Contains(errStr, "no redirect from xbox sisu"),
		strings.Contains(errStr, "redirect chain ended"),
		strings.Contains(errStr, "too many redirects"):
		return &CheckError{Category: "AUTH_FAILED", Message: "Xbox auth chain failed", Detail: errStr}

	case strings.Contains(errStr, "429"):
		return &CheckError{Category: "RATE_LIMITED", Message: "Rate limited by API", Detail: errStr}

	case strings.Contains(errStr, "mc auth failed (status"):
		return &CheckError{Category: "MC_REJECTED", Message: "Minecraft services rejected token", Detail: errStr}

	case strings.Contains(errStr, "login.live.com oauth rejected"),
		strings.Contains(errStr, "login page"):
		return &CheckError{Category: "EXPIRED", Message: "Cookie expired or invalid", Detail: errStr}

	case strings.Contains(errStr, "timeout"),
		strings.Contains(errStr, "deadline"),
		strings.Contains(errStr, "Timeout"):
		return &CheckError{Category: "TIMEOUT", Message: "Request timed out", Detail: errStr}

	case strings.Contains(errStr, "connection refused"),
		strings.Contains(errStr, "no such host"),
		strings.Contains(errStr, "reset by peer"),
		strings.Contains(errStr, "broken pipe"):
		return &CheckError{Category: "NETWORK", Message: "Network error", Detail: errStr}

	case strings.Contains(errStr, "parse error"):
		return &CheckError{Category: "PARSE_ERROR", Message: "Failed to parse cookie file", Detail: errStr}

	default:
		return &CheckError{Category: "UNKNOWN", Message: errStr, Detail: errStr}
	}
}

func categorizeAuthError(err error) *CheckError {
	if err == nil {
		return nil
	}
	errStr := err.Error()

	switch {
	case strings.Contains(errStr, "password is incorrect"),
		strings.Contains(errStr, "account doesn't exist"):
		return &CheckError{Category: "INVALID_CREDENTIALS", Message: "Wrong email or password", Detail: errStr}

	case strings.Contains(errStr, "account locked"),
		strings.Contains(errStr, "30_day_lockout"),
		strings.Contains(errStr, "account is locked"):
		return &CheckError{Category: "LOCKED", Message: "Account is locked", Detail: errStr}

	case strings.Contains(errStr, "rate_limited"):
		return &CheckError{Category: "RATE_LIMITED", Message: "Rate limited", Detail: errStr}

	case strings.Contains(errStr, "verification_required"),
		strings.Contains(errStr, "2fa_required"):
		return &CheckError{Category: "VERIFY_REQUIRED", Message: "Verification required", Detail: errStr}

	case strings.Contains(errStr, "PPFT token not found"),
		strings.Contains(errStr, "URL POST not found"):
		return &CheckError{Category: "PAGE_CHANGED", Message: "Microsoft login page structure changed", Detail: errStr}

	case strings.Contains(errStr, "no token returned"):
		return &CheckError{Category: "SILENT_FAIL", Message: "Login failed silently", Detail: errStr}

	case strings.Contains(errStr, "timeout"),
		strings.Contains(errStr, "deadline"),
		strings.Contains(errStr, "Timeout"):
		return &CheckError{Category: "TIMEOUT", Message: "Request timed out", Detail: errStr}

	case strings.Contains(errStr, "connection refused"),
		strings.Contains(errStr, "no such host"),
		strings.Contains(errStr, "reset by peer"),
		strings.Contains(errStr, "broken pipe"):
		return &CheckError{Category: "NETWORK", Message: "Network connection failed", Detail: errStr}

	default:
		return &CheckError{Category: "UNKNOWN", Message: errStr, Detail: errStr}
	}
}

func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "reset by peer") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "deadline exceeded")
}

func logError(filename, label string, err error, extra ...string) {
	if err == nil {
		return
	}
	cat := categorizeCookieError(err)
	if cat == nil {
		cat = &CheckError{Category: "UNKNOWN", Message: err.Error()}
	}
	ts := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] [%s] %s: %s", ts, cat.Category, label, cat.Detail)
	if len(extra) > 0 {
		line += " | " + strings.Join(extra, " | ")
	}
	writeToFile(filename, line)
}

func logNetworkError(filename, label string, err error) {
	if err == nil {
		return
	}
	if isNetworkError(err) {
		logError(filename, label, err)
	}
}

func safeWrite(filename, content string) {
	err := retryWrite(filename, content, 3)
	if err != nil {
		fmt.Fprintf(os.Stderr, "CRITICAL: Failed to write %s after retries: %v\n", filename, err)
	}
}

func retryWrite(filename, content string, attempts int) error {
	var lastErr error
	for i := 0; i < attempts; i++ {
		f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
			continue
		}
		_, writeErr := fmt.Fprintln(f, content)
		f.Close()
		if writeErr == nil {
			return nil
		}
		lastErr = writeErr
		time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
	}
	return lastErr
}

func wrapNetError(err error) error {
	if err == nil {
		return nil
	}
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "connection refused"):
		return fmt.Errorf("connection refused: %w", err)
	case strings.Contains(errStr, "no such host"):
		return fmt.Errorf("dns resolution failed: %w", err)
	case strings.Contains(errStr, "connection reset"):
		return fmt.Errorf("connection reset: %w", err)
	case strings.Contains(errStr, "broken pipe"):
		return fmt.Errorf("broken pipe: %w", err)
	case strings.Contains(errStr, "i/o timeout"):
		return fmt.Errorf("i/o timeout: %w", err)
	}
	return err
}
