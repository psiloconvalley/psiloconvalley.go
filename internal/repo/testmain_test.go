// internal/repo/testmain_test.go
//
// Shared test harness for the repo package.
//
// Opens one connection to the local Postgres database, runs all migrations
// once, and exposes the connection to every test via testDB.
//
// Each test should use withTx(t) to run inside a transaction that gets
// rolled back on test completion — zero state leaks between tests.
//
// If no test database is reachable, tests are skipped (not failed) so that
// `go test ./...` on a machine without Postgres doesn't break the build.
//
// SAFETY: refuses to run against any DSN containing production host markers
// (rlwy.net, railway.app, .railway.). Prevents accidental data loss.
package repo

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"psiloconvalley/internal/migrate"
)

const defaultTestDSN = "postgresql://postgres:localdev@localhost:5432/psiloconvalley?sslmode=disable"

// Production host markers — if any of these appear in the DSN, tests refuse
// to run. Matches the prod guard in internal/app/db.go.
var prodHostMarkers = []string{"rlwy.net", "railway.app", ".railway."}

// testDB is the shared package-level connection. Nil until TestMain opens it.
// nil means "no database available" and tests should skip.
var testDB *sql.DB

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = defaultTestDSN
	}

	// Hard refuse against production DSNs. No ALLOW_PROD escape hatch for tests.
	lower := strings.ToLower(dsn)
	for _, marker := range prodHostMarkers {
		if strings.Contains(lower, marker) {
			fmt.Fprintf(os.Stderr,
				"repo tests refuse to run against production DSN (%s). Set TEST_DATABASE_URL to a local database.\n",
				marker,
			)
			os.Exit(1)
		}
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "test harness: sql.Open failed: %v (tests will skip)\n", err)
		os.Exit(m.Run())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "test harness: ping failed: %v (tests will skip)\n", err)
		db.Close()
		os.Exit(m.Run())
	}

	// Run migrations. If this fails, hard-stop — we don't want tests running
	// against a half-migrated schema.
	if err := migrate.Run(db, "../../migrations"); err != nil {
		fmt.Fprintf(os.Stderr, "test harness: migrate.Run failed: %v\n", err)
		db.Close()
		os.Exit(1)
	}

	testDB = db
	code := m.Run()
	db.Close()
	os.Exit(code)
}

// withTx runs fn inside a transaction that is always rolled back.
// If no test DB is available, the test is skipped.
//
// Pattern:
//
//	func TestX(t *testing.T) {
//	    withTx(t, func(tx *sql.Tx) {
//	        repo := &InvoiceViewRepo{db: tx}
//	        // ... exercise repo ...
//	    })
//	}
func withTx(t *testing.T, fn func(tx *sql.Tx)) {
	t.Helper()
	if testDB == nil {
		t.Skip("no test database available (set TEST_DATABASE_URL or start local Postgres)")
	}

	tx, err := testDB.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			t.Errorf("Rollback: %v", err)
		}
	}()

	fn(tx)
}

// mustCreateInvoice inserts a minimal invoice row inside the test transaction
// and returns its ID. Needed because invoice_views has a FK to invoices.
//
// We don't care about invoice contents — just that the FK target exists.
func mustCreateInvoice(t *testing.T, tx *sql.Tx) int64 {
	t.Helper()
	var id int64
	err := tx.QueryRowContext(context.Background(), `
		INSERT INTO invoices (client_name, invoice_number, issue_date, currency, status)
		VALUES ('Test Client', $1, now(), 'USD', 'draft')
		RETURNING id
	`, fmt.Sprintf("TEST-%d", time.Now().UnixNano())).Scan(&id)
	if err != nil {
		t.Fatalf("mustCreateInvoice: %v", err)
	}
	return id
}
