package service

import "strings"

const (
	defaultLocale = "en"
	zhHansLocale  = "zh-Hans"
)

func normalizeLocale(locale string) string {
	trimmed := strings.TrimSpace(locale)
	if trimmed == "" {
		return defaultLocale
	}

	lower := strings.ToLower(trimmed)
	switch lower {
	case "en", "en-us", "en-sg", "en-gb":
		return defaultLocale
	case "zh-hans", "zh-cn", "zh", "zh_sg":
		return zhHansLocale
	default:
		return defaultLocale
	}
}
