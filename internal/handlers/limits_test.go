package handlers

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"psiloconvalley/internal/catalog"
	"psiloconvalley/internal/repo"
)

// ── Fake usage store ─────────────────────────────────────────────────
// Data storage only. No business logic. The fake remembers counts in a
// map and returns them on Get. That's it.

type fakeUsageStore struct {
	counts map[string]int
	getErr error
}

func newFakeUsage() *fakeUsageStore {
	return &fakeUsageStore{counts: make(map[string]int)}
}

func (f *fakeUsageStore) key(userID int64, feature string) string {
	return fmt.Sprintf("%d:%s", userID, feature)
}

func (f *fakeUsageStore) Get(_ context.Context, userID int64, feature string) (int, error) {
	if f.getErr != nil {
		return 0, f.getErr
	}
	return f.counts[f.key(userID, feature)], nil
}

func (f *fakeUsageStore) Increment(_ context.Context, userID int64, feature string) error {
	f.counts[f.key(userID, feature)]++
	return nil
}

// Compile-time guarantee: if usageStore changes, this file won't compile.
var _ usageStore = (*fakeUsageStore)(nil)

// ── Tests ────────────────────────────────────────────────────────────

func TestUsageAllowedFor_NilUser(t *testing.T) {
	got := usageAllowedFor(context.Background(), newFakeUsage(), nil, "invoices", 5, 30)
	if got {
		t.Fatal("nil user should be denied")
	}
}

func TestUsageAllowedFor_GrandfatheredUser_OnPro_IsUnlimited(t *testing.T) {
	// User ID 6 is in the grandfather allowlist (founding customer).
	// On Pro, they get unlimited access regardless of count.
	store := newFakeUsage()
	store.counts["6:invoices"] = 999
	user := &repo.User{ID: 6, Plan: catalog.PlanPro}

	got := usageAllowedFor(context.Background(), store, user, "invoices", 5, 30)
	if !got {
		t.Fatal("grandfathered user on Pro should be unlimited, even at 999")
	}
}

func TestUsageAllowedFor_GrandfatheredUser_OnFree_IsNotExempt(t *testing.T) {
	// Grandfather status requires an active Pro plan. If a grandfathered
	// user drops to Free, normal free-tier limits apply.
	store := newFakeUsage()
	store.counts["6:invoices"] = 5
	user := &repo.User{ID: 6, Plan: catalog.PlanFree}

	got := usageAllowedFor(context.Background(), store, user, "invoices", 5, 30)
	if got {
		t.Fatal("grandfather requires Pro plan; Free user at limit should be blocked")
	}
}

func TestUsageAllowedFor_NonGrandfatheredLowID_IsNotExempt(t *testing.T) {
	// Regression trap: the old implementation granted unlimited access to
	// any userID <= 10 on Pro. The new allowlist must NOT exempt low IDs
	// that aren't explicitly listed. If this test fails, someone reverted
	// to range-based grandfathering.
	store := newFakeUsage()
	store.counts["1:invoices"] = 30
	user := &repo.User{ID: 1, Plan: catalog.PlanPro}

	got := usageAllowedFor(context.Background(), store, user, "invoices", 5, 30)
	if got {
		t.Fatal("low user IDs are not grandfathered by default — should be blocked at pro limit")
	}
}

func TestUsageAllowedFor_ProMax_IsUnlimited(t *testing.T) {
	store := newFakeUsage()
	store.counts["99:invoices"] = 999
	user := &repo.User{ID: 99, Plan: catalog.PlanProMax}

	got := usageAllowedFor(context.Background(), store, user, "invoices", 5, 30)
	if !got {
		t.Fatal("ProMax should always be unlimited")
	}
}

func TestUsageAllowedFor_FreeUnderLimit(t *testing.T) {
	store := newFakeUsage()
	store.counts["99:invoices"] = 3
	user := &repo.User{ID: 99, Plan: catalog.PlanFree}

	got := usageAllowedFor(context.Background(), store, user, "invoices", 5, 30)
	if !got {
		t.Fatal("free user at 3/5 should be allowed")
	}
}

func TestUsageAllowedFor_FreeAtLimit(t *testing.T) {
	store := newFakeUsage()
	store.counts["99:invoices"] = 5
	user := &repo.User{ID: 99, Plan: catalog.PlanFree}

	got := usageAllowedFor(context.Background(), store, user, "invoices", 5, 30)
	if got {
		t.Fatal("free user at 5/5 should be blocked")
	}
}

func TestUsageAllowedFor_FreeOverLimit(t *testing.T) {
	store := newFakeUsage()
	store.counts["99:invoices"] = 7
	user := &repo.User{ID: 99, Plan: catalog.PlanFree}

	got := usageAllowedFor(context.Background(), store, user, "invoices", 5, 30)
	if got {
		t.Fatal("free user at 7/5 should be blocked")
	}
}

func TestUsageAllowedFor_ProUnderLimit(t *testing.T) {
	store := newFakeUsage()
	store.counts["99:invoices"] = 29
	user := &repo.User{ID: 99, Plan: catalog.PlanPro}

	got := usageAllowedFor(context.Background(), store, user, "invoices", 5, 30)
	if !got {
		t.Fatal("pro user at 29/30 should be allowed")
	}
}

func TestUsageAllowedFor_ProAtLimit(t *testing.T) {
	store := newFakeUsage()
	store.counts["99:invoices"] = 30
	user := &repo.User{ID: 99, Plan: catalog.PlanPro}

	got := usageAllowedFor(context.Background(), store, user, "invoices", 5, 30)
	if got {
		t.Fatal("pro user at 30/30 should be blocked")
	}
}

func TestUsageAllowedFor_ZeroLimitMeansUnlimited(t *testing.T) {
	store := newFakeUsage()
	store.counts["99:reports"] = 100
	user := &repo.User{ID: 99, Plan: catalog.PlanFree}

	// FreeReportLimit is 0, which means "no cap" for this feature
	got := usageAllowedFor(context.Background(), store, user, "reports", 0, 0)
	if !got {
		t.Fatal("limit of 0 means unlimited — should always allow")
	}
}

func TestUsageAllowedFor_DBErrorFailsOpen(t *testing.T) {
	store := newFakeUsage()
	store.getErr = errors.New("connection refused")
	user := &repo.User{ID: 99, Plan: catalog.PlanFree}

	got := usageAllowedFor(context.Background(), store, user, "invoices", 5, 30)
	if !got {
		t.Fatal("DB error should fail open — quota is commercial, not security")
	}
}
