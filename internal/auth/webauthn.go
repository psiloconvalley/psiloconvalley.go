// internal/auth/webauthn.go
// WebAuthn instance initialization.
// One instance shared across all handlers — configured at startup.
package auth

import (
	"fmt"
	"os"

	"github.com/go-webauthn/webauthn/webauthn"
)

// WebAuthn is the shared WebAuthn instance used by all passkey handlers.
var WebAuthn *webauthn.WebAuthn

// InitWebAuthn initializes the WebAuthn instance from environment config.
// Must be called once at startup after InitSessionSecret().
func InitWebAuthn() error {
	rpID := os.Getenv("WEBAUTHN_RP_ID")
	if rpID == "" {
		rpID = "psiloconvalley.com" // production default
	}

	rpOrigin := os.Getenv("WEBAUTHN_ORIGIN")
	if rpOrigin == "" {
		rpOrigin = "https://psiloconvalley.com" // production default
	}

	wconfig := &webauthn.Config{
		RPDisplayName: "PsiloConValley",
		RPID:          rpID,
		RPOrigins:     []string{rpOrigin},
	}

	var err error
	WebAuthn, err = webauthn.New(wconfig)
	if err != nil {
		return fmt.Errorf("webauthn init: %w", err)
	}
	return nil
}
