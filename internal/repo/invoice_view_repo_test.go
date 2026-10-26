// internal/repo/invoice_view_repo_test.go
package repo

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestInvoiceViewRepo_Log_InsertsRow(t *testing.T) {
	withTx(t, func(tx *sql.Tx) {
		ctx := context.Background()
		invID := mustCreateInvoice(t, tx)
		repo := &InvoiceViewRepo{db: tx}

		ua := "Mozilla/5.0 Chrome/120"
		ref := "https://example.com/"
		err := repo.Log(ctx, InvoiceView{
			InvoiceID:  invID,
			EventType:  "html",
			ViewerType: "public_link",
			UserAgent:  &ua,
			Referer:    &ref,
		})
		if err != nil {
			t.Fatalf("Log: %v", err)
		}

		// Verify the row landed.
		var count int
		err = tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM invoice_views WHERE invoice_id = $1`,
			invID,
		).Scan(&count)
		if err != nil {
			t.Fatalf("count query: %v", err)
		}
		if count != 1 {
			t.Errorf("want 1 row, got %d", count)
		}
	})
}

func TestInvoiceViewRepo_Log_RejectsBadEventType(t *testing.T) {
	withTx(t, func(tx *sql.Tx) {
		ctx := context.Background()
		invID := mustCreateInvoice(t, tx)
		repo := &InvoiceViewRepo{db: tx}

		err := repo.Log(ctx, InvoiceView{
			InvoiceID:  invID,
			EventType:  "bogus", // not in CHECK constraint
			ViewerType: "public_link",
		})
		if err == nil {
			t.Fatal("expected CHECK constraint violation, got nil")
		}
	})
}

func TestInvoiceViewRepo_Log_RejectsBadViewerType(t *testing.T) {
	withTx(t, func(tx *sql.Tx) {
		ctx := context.Background()
		invID := mustCreateInvoice(t, tx)
		repo := &InvoiceViewRepo{db: tx}

		err := repo.Log(ctx, InvoiceView{
			InvoiceID:  invID,
			EventType:  "html",
			ViewerType: "owner", // not in CHECK constraint
		})
		if err == nil {
			t.Fatal("expected CHECK constraint violation, got nil")
		}
	})
}

func TestInvoiceViewRepo_LastHumanViewFor_NoRows(t *testing.T) {
	withTx(t, func(tx *sql.Tx) {
		ctx := context.Background()
		invID := mustCreateInvoice(t, tx)
		repo := &InvoiceViewRepo{db: tx}

		got, err := repo.LastHumanViewFor(ctx, invID)
		if err != nil {
			t.Fatalf("LastHumanViewFor: %v", err)
		}
		if got != nil {
			t.Errorf("want nil, got %+v", got)
		}
	})
}

func TestInvoiceViewRepo_LastHumanViewFor_ReturnsMostRecent(t *testing.T) {
	withTx(t, func(tx *sql.Tx) {
		ctx := context.Background()
		invID := mustCreateInvoice(t, tx)
		repo := &InvoiceViewRepo{db: tx}

		// Insert three public_link rows at increasing timestamps.
		// We can't easily control now() so we insert then UPDATE the
		// viewed_at column to known offsets.
		for i, offset := range []time.Duration{-3 * time.Hour, -2 * time.Hour, -1 * time.Hour} {
			ua := "Chrome"
			if err := repo.Log(ctx, InvoiceView{
				InvoiceID:  invID,
				EventType:  []string{"html", "pdf", "pay"}[i],
				ViewerType: "public_link",
				UserAgent:  &ua,
			}); err != nil {
				t.Fatalf("Log %d: %v", i, err)
			}
			_, err := tx.ExecContext(ctx, `
				UPDATE invoice_views
				   SET viewed_at = now() + $1
				 WHERE invoice_id = $2
				   AND event_type = $3
			`, offset, invID, []string{"html", "pdf", "pay"}[i])
			if err != nil {
				t.Fatalf("update viewed_at %d: %v", i, err)
			}
		}

		got, err := repo.LastHumanViewFor(ctx, invID)
		if err != nil {
			t.Fatalf("LastHumanViewFor: %v", err)
		}
		if got == nil {
			t.Fatal("want row, got nil")
		}
		if got.EventType != "pay" {
			t.Errorf("want most recent (pay), got %q", got.EventType)
		}
	})
}

func TestInvoiceViewRepo_LastHumanViewFor_IgnoresBotRows(t *testing.T) {
	withTx(t, func(tx *sql.Tx) {
		ctx := context.Background()
		invID := mustCreateInvoice(t, tx)
		repo := &InvoiceViewRepo{db: tx}

		ua := "Applebot/0.1"
		if err := repo.Log(ctx, InvoiceView{
			InvoiceID:  invID,
			EventType:  "html",
			ViewerType: "bot",
			UserAgent:  &ua,
		}); err != nil {
			t.Fatalf("Log bot: %v", err)
		}

		got, err := repo.LastHumanViewFor(ctx, invID)
		if err != nil {
			t.Fatalf("LastHumanViewFor: %v", err)
		}
		if got != nil {
			t.Errorf("want nil (bot rows excluded), got %+v", got)
		}
	})
}

func TestInvoiceViewRepo_LastHumanViewForInvoices_EmptyInput(t *testing.T) {
	withTx(t, func(tx *sql.Tx) {
		ctx := context.Background()
		repo := &InvoiceViewRepo{db: tx}

		got, err := repo.LastHumanViewForInvoices(ctx, nil)
		if err != nil {
			t.Fatalf("empty slice: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("want empty map, got %d entries", len(got))
		}

		got, err = repo.LastHumanViewForInvoices(ctx, []int64{})
		if err != nil {
			t.Fatalf("empty slice: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("want empty map, got %d entries", len(got))
		}
	})
}

func TestInvoiceViewRepo_LastHumanViewForInvoices_BatchReturnsNewestPerInvoice(t *testing.T) {
	withTx(t, func(tx *sql.Tx) {
		ctx := context.Background()
		repo := &InvoiceViewRepo{db: tx}

		invA := mustCreateInvoice(t, tx)
		invB := mustCreateInvoice(t, tx)
		invC := mustCreateInvoice(t, tx) // will have no views at all

		ua := "Chrome"

		// invA: two public_link rows, newest should win
		for i, et := range []string{"html", "pay"} {
			if err := repo.Log(ctx, InvoiceView{
				InvoiceID: invA, EventType: et, ViewerType: "public_link", UserAgent: &ua,
			}); err != nil {
				t.Fatalf("Log invA %d: %v", i, err)
			}
		}
		_, err := tx.ExecContext(ctx,
			`UPDATE invoice_views SET viewed_at = now() - interval '2 hours' WHERE invoice_id = $1 AND event_type = 'html'`,
			invA)
		if err != nil {
			t.Fatalf("backdate invA html: %v", err)
		}

		// invB: one bot row only (should be excluded from result)
		botUA := "Applebot/0.1"
		if err := repo.Log(ctx, InvoiceView{
			InvoiceID: invB, EventType: "html", ViewerType: "bot", UserAgent: &botUA,
		}); err != nil {
			t.Fatalf("Log invB bot: %v", err)
		}

		got, err := repo.LastHumanViewForInvoices(ctx, []int64{invA, invB, invC})
		if err != nil {
			t.Fatalf("LastHumanViewForInvoices: %v", err)
		}

		// invA present, newest = "pay"
		rowA, ok := got[invA]
		if !ok {
			t.Fatal("want entry for invA, missing")
		}
		if rowA.EventType != "pay" {
			t.Errorf("invA: want newest event_type=pay, got %q", rowA.EventType)
		}

		// invB absent (bot-only)
		if _, ok := got[invB]; ok {
			t.Error("invB should be absent (bot-only), present in map")
		}

		// invC absent (no views at all)
		if _, ok := got[invC]; ok {
			t.Error("invC should be absent (no views), present in map")
		}
	})
}
