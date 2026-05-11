package util

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func Money(cents int64) string {
	return fmt.Sprintf("$%.2f", float64(cents)/100.0)
}

func FormatCentsForInput(cents int64) string {
	if cents == 0 {
		return ""
	}
	return fmt.Sprintf("%.2f", float64(cents)/100.0)
}

func BpsToPercent(bps int64) string {
	if bps == 0 {
		return ""
	}
	return fmt.Sprintf("%.2f", float64(bps)/100.0)
}

func ParseCents(val string) int64 {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0
	}
	return int64(math.Round(f * 100))
}

func ParseBps(val string) int64 {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0
	}
	return int64(math.Round(f * 100))
}
