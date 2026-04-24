package main

import "os"

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
)

var colorEnabled bool

func init() {
	fi, err := os.Stdout.Stat()
	colorEnabled = err == nil && fi.Mode()&os.ModeCharDevice != 0 && os.Getenv("NO_COLOR") == ""
}

func clr(code, s string) string {
	if !colorEnabled {
		return s
	}
	return code + s + ansiReset
}

func bold(s string) string    { return clr(ansiBold, s) }
func cRed(s string) string    { return clr(ansiRed, s) }
func cGreen(s string) string  { return clr(ansiGreen, s) }
func cYellow(s string) string { return clr(ansiYellow, s) }
func cCyan(s string) string   { return clr(ansiCyan, s) }
func cDim(s string) string    { return clr(ansiDim, s) }
