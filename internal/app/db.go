package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

var DB *sql.DB

// prodHostMarkers identifies DSNs pointing at Railway production infrastructure.
// Any match requires ALLOW_PROD=1 to proceed. This prevents local dev commands
// (migrations, tests, go run) from accidentally hitting production.
var prodHostMarkers = []string{
	"rlwy.net",
	"railway.app",
	".railway.",
}

// ProdGuardVerdict is the outcome of checking a DSN against prod markers.
type ProdGuardVerdict int

const (
	// GuardAllow means the DSN is safe to use (not prod, or prod with ALLOW_PROD=1).
	GuardAllow ProdGuardVerdict = iota
	// GuardAllowWithWarning means the DSN is prod but ALLOW_PROD=1 is set.
	GuardAllowWithWarning
	// GuardRefuse means the DSN is prod and ALLOW_PROD is not set. Caller must abort.
	GuardRefuse
)

// CheckProdGuard is pure: given a DSN and the ALLOW_PROD flag value, decide
// whether to allow, allow-with-warning, or refuse the connection.
// Returns the matched marker string when prod is detected (for logging/errors).
func CheckProdGuard(dsn, allowProd string) (ProdGuardVerdict, string) {
	lower := strings.ToLower(dsn)
	for _, marker := range prodHostMarkers {
		if strings.Contains(lower, marker) {
			if allowProd == "1" {
				return GuardAllowWithWarning, marker
			}
			return GuardRefuse, marker
		}
	}
	return GuardAllow, ""
}

// enforceProdGuard applies the verdict: logs warning or aborts process.
func enforceProdGuard(dsn string) {
	verdict, marker := CheckProdGuard(dsn, os.Getenv("ALLOW_PROD"))
	switch verdict {
	case GuardRefuse:
		log.Fatal("refusing to connect: DATABASE_URL points at production (" + marker + "). " +
			"To override, set ALLOW_PROD=1. " +
			"To use local, point DATABASE_URL at localhost (e.g., docker postgres on :5432).")
	case GuardAllowWithWarning:
		slog.Warn("CONNECTING TO PRODUCTION DATABASE", "marker", marker, "allow_prod", "1")
	}
}

func InitDB() {
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	enforceProdGuard(dsn)

	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}

	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.PingContext(ctx); err != nil {
		log.Fatal(fmt.Errorf("database ping failed: %w", err))
	}

	DB = conn
	slog.Info("database connected")
}

func CloseDB() {
	if DB != nil {
		DB.Close()
	}
}
