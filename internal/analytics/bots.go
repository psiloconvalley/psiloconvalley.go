// Package analytics contains view-tracking logic for invoices.
//
// This file defines bot detection. Patterns are matched case-insensitively
// as substrings against the User-Agent header.
//
// Critical: applebot and facebookexternalhit must be filtered. iMessage
// link previews fetch URLs with these UAs the moment a link is sent.
// Without this filter, every invoice would register as "opened" instantly
// and the feature would be worse than nothing.
package analytics

import "strings"

// botPatterns are case-insensitive substrings. If any appears in the UA,
// the request is treated as a bot.
var botPatterns = []string{
	"googlebot",
	"bingbot",
	"applebot",
	"facebookexternalhit",
	"twitterbot",
	"linkedinbot",
	"slackbot",
	"discordbot",
	"telegrambot",
	"whatsapp",
	"curl",
	"wget",
	"python-requests",
	"go-http-client",
	"java/",
	"okhttp",
	"headlesschrome",
	"phantomjs",
	"puppeteer",
	"ahrefsbot",
	"semrushbot",
	"mj12bot",
	"dotbot",
}

// isBot reports whether the given User-Agent string looks like a bot or
// automated client. An empty UA is treated as a bot — real browsers always
// send one.
func isBot(userAgent string) bool {
	if userAgent == "" {
		return true
	}
	lower := strings.ToLower(userAgent)
	for _, p := range botPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
