package analytics

import "testing"

func TestDecideView(t *testing.T) {
	tests := []struct {
		name           string
		isOwner        bool
		isAnonMatch    bool
		userAgent      string
		wantShouldLog  bool
		wantViewerType string
	}{
		// Owner paths — never log, regardless of UA.
		{
			name:          "owner with chrome UA skips",
			isOwner:       true,
			userAgent:     "Mozilla/5.0 (Macintosh) Chrome/120",
			wantShouldLog: false,
		},
		{
			name:          "owner with bot UA skips",
			isOwner:       true,
			userAgent:     "Applebot/0.1",
			wantShouldLog: false,
		},
		{
			name:          "owner with empty UA skips",
			isOwner:       true,
			userAgent:     "",
			wantShouldLog: false,
		},

		// Anonymous owner (cookie match) — never log.
		{
			name:          "anon token match with chrome UA skips",
			isAnonMatch:   true,
			userAgent:     "Mozilla/5.0 Chrome/120",
			wantShouldLog: false,
		},
		{
			name:          "anon token match with bot UA skips",
			isAnonMatch:   true,
			userAgent:     "facebookexternalhit/1.1",
			wantShouldLog: false,
		},

		// Public viewer with real browser UAs — log as public_link.
		{
			name:           "public chrome logs as public_link",
			userAgent:      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
			wantShouldLog:  true,
			wantViewerType: ViewerPublicLink,
		},
		{
			name:           "public safari iphone logs as public_link",
			userAgent:      "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Version/17.0 Mobile/15E148 Safari/604.1",
			wantShouldLog:  true,
			wantViewerType: ViewerPublicLink,
		},
		{
			name:           "public firefox logs as public_link",
			userAgent:      "Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
			wantShouldLog:  true,
			wantViewerType: ViewerPublicLink,
		},

		// Bot UAs — log as bot. Critical: applebot + facebookexternalhit.
		{
			name:           "applebot logs as bot",
			userAgent:      "Mozilla/5.0 (compatible; Applebot/0.1; +http://www.apple.com/go/applebot)",
			wantShouldLog:  true,
			wantViewerType: ViewerBot,
		},
		{
			name:           "facebookexternalhit logs as bot",
			userAgent:      "facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)",
			wantShouldLog:  true,
			wantViewerType: ViewerBot,
		},
		{
			name:           "googlebot logs as bot",
			userAgent:      "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			wantShouldLog:  true,
			wantViewerType: ViewerBot,
		},
		{
			name:           "slackbot logs as bot",
			userAgent:      "Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)",
			wantShouldLog:  true,
			wantViewerType: ViewerBot,
		},
		{
			name:           "whatsapp preview logs as bot",
			userAgent:      "WhatsApp/2.23.24.76 A",
			wantShouldLog:  true,
			wantViewerType: ViewerBot,
		},
		{
			name:           "curl logs as bot",
			userAgent:      "curl/8.1.2",
			wantShouldLog:  true,
			wantViewerType: ViewerBot,
		},
		{
			name:           "go-http-client logs as bot",
			userAgent:      "Go-http-client/1.1",
			wantShouldLog:  true,
			wantViewerType: ViewerBot,
		},
		{
			name:           "empty UA logs as bot",
			userAgent:      "",
			wantShouldLog:  true,
			wantViewerType: ViewerBot,
		},

		// Case insensitivity.
		{
			name:           "APPLEBOT uppercase logs as bot",
			userAgent:      "APPLEBOT/0.1",
			wantShouldLog:  true,
			wantViewerType: ViewerBot,
		},
		{
			name:           "AppleBot mixed case logs as bot",
			userAgent:      "AppleBot/0.1",
			wantShouldLog:  true,
			wantViewerType: ViewerBot,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DecideView(tc.isOwner, tc.isAnonMatch, tc.userAgent)
			if got.ShouldLog != tc.wantShouldLog {
				t.Errorf("ShouldLog: got %v, want %v", got.ShouldLog, tc.wantShouldLog)
			}
			if tc.wantShouldLog && got.ViewerType != tc.wantViewerType {
				t.Errorf("ViewerType: got %q, want %q", got.ViewerType, tc.wantViewerType)
			}
			if !tc.wantShouldLog && got.ViewerType != "" {
				t.Errorf("ViewerType should be empty when ShouldLog=false, got %q", got.ViewerType)
			}
		})
	}
}
