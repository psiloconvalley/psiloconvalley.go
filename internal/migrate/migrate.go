// internal/migrate/migrate.go
//
// Automatic database migration engine.
//
// Reads .sql files from a directory, applies them in lexicographic order,
// and tracks applied migrations in a schema_migrations table.
//
// Design principles:
//   - Zero external dependencies (stdlib + database/sql only)
//   - Idempotent — safe to run on every startup
//   - Tracked — schema_migrations records what has been applied
//   - Ordered — filename sort determines sequence
//   - Fail-fast — duplicate prefixes or SQL errors halt startup
//   - Logged — every action is visible in Railway logs
package migrate

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Run applies all pending migrations from dir to db.
// Call once from main() after database connection is established.
//
// Behavior:
//   1. Creates schema_migrations table if it does not exist
//   2. Reads all *.sql files from dir
//   3. Detects duplicate numeric prefixes — returns error (app will not start)
//   4. Skips files already recorded in schema_migrations
//   5. Applies pending files in lexicographic order within a transaction each
//   6. Records each applied file in schema_migrations
//
// Returns nil if all migrations are applied successfully.
// Returns an error if any migration fails — the app should log.Fatal.
func Run(db *sql.DB, dir string) error {
	start := time.Now()

	// ── Ensure tracking table exists ─────────────────────────────────
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename   TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	if err != nil {
		return fmt.Errorf("migrate: create tracking table: %w", err)
	}

	// ── Read migration files ─────────────────────────────────────────
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("migrate: read directory %q: %w", dir, err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	if len(files) == 0 {
		slog.Info("migrate: no .sql files found", "dir", dir)
		return nil
	}

	// ── Detect duplicate numeric prefixes ────────────────────────────
	// Extract the prefix before the first underscore (e.g., "011" from "011_add_audit_logs.sql").
	// Two files with the same prefix means ambiguous ordering — refuse to start.
	prefixes := make(map[string]string) // prefix → filename
	for _, f := range files {
		parts := strings.SplitN(f, "_", 2)
		prefix := parts[0]
		if existing, ok := prefixes[prefix]; ok {
			return fmt.Errorf("migrate: duplicate prefix %q: %q and %q — rename one before deploying", prefix, existing, f)
		}
		prefixes[prefix] = f
	}

	// ── Load already-applied set ─────────────────────────────────────
	rows, err := db.Query(`SELECT filename FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("migrate: query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("migrate: scan applied migration: %w", err)
		}
		applied[name] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("migrate: iterate applied migrations: %w", err)
	}

	// ── Apply pending migrations ─────────────────────────────────────
	var appliedCount int
	for _, f := range files {
		if applied[f] {
			continue
		}

		sqlBytes, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			return fmt.Errorf("migrate: read %q: %w", f, err)
		}

		sqlStr := string(sqlBytes)
		if strings.TrimSpace(sqlStr) == "" {
			slog.Warn("migrate: skipping empty file", "file", f)
			continue
		}

		// Execute migration in a transaction.
		// If the SQL fails, the transaction rolls back and the app does not start.
		// This prevents half-applied migrations.
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("migrate: begin tx for %q: %w", f, err)
		}

		if _, err := tx.Exec(sqlStr); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrate: execute %q: %w", f, err)
		}

		if _, err := tx.Exec(`INSERT INTO schema_migrations (filename) VALUES ($1)`, f); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrate: record %q: %w", f, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migrate: commit %q: %w", f, err)
		}

		slog.Info("migrate: applied", "file", f)
		appliedCount++
	}

	slog.Info("migrate: complete",
		"applied", appliedCount,
		"total", len(files),
		"skipped", len(files)-appliedCount,
		"elapsed", time.Since(start).Round(time.Millisecond).String(),
	)

	return nil
}
