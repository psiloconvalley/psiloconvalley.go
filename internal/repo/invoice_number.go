package repo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// =====================================================================
// Invoice Number Domain Service
//
// This file owns ALL logic related to invoice number generation,
// normalization, conflict resolution, and uniqueness enforcement.
//
// Responsibilities:
//   - System-controlled invoice ID generation
//   - Sanitization of user-provided values
//   - Deterministic resolution between user input and system values
//   - Collision recovery against the database
//
// Goals:
//   - The handler must never construct or mutate invoice numbers.
//   - The database constraint is treated as a safety net, not a
//     primary mechanism for uniqueness.
//   - All logic is pure where possible and easy to test.
// =====================================================================

// invoiceNumberMaxAttempts caps how many times we will try to resolve
// a unique invoice number before giving up.
//
// Five is more than enough in practice. If we ever exceed five, something
// is structurally wrong and an error is preferable to silent retries.
const invoiceNumberMaxAttempts = 5

// GenerateInvoiceNumber creates a system-authoritative invoice number.
//
// Format:
//
//	INV-{unix_ms}-{4 hex chars}
//
// Properties:
//   - Globally unique without database round-trips
//   - Sortable by time
//   - Compact and human-readable
//   - Independent of user input
func GenerateInvoiceNumber() string {

	timestamp := time.Now().UnixNano() / int64(time.Millisecond)

	entropy := make([]byte, 2)
	_, _ = rand.Read(entropy)

	return fmt.Sprintf(
		"INV-%d-%s",
		timestamp,
		hex.EncodeToString(entropy),
	)
}

// NormalizeInvoiceNumber sanitizes user-provided input.
//
// Rules:
//   - Trim leading/trailing whitespace
//   - Collapse internal whitespace to single spaces
//   - Empty strings stay empty (signals "no value provided")
//
// We deliberately do NOT mutate user-supplied IDs beyond cleanup.
// Identity decisions belong to higher-level resolution.
func NormalizeInvoiceNumber(input string) string {

	cleaned := strings.TrimSpace(input)
	cleaned = strings.Join(strings.Fields(cleaned), " ")

	return cleaned
}

// ResolveInvoiceNumber decides which invoice number should be used.
//
// Behavior:
//   - If user provided a value → return the normalized value
//   - If user provided nothing → return a system-generated value
//
// This function is pure. It performs no I/O.
func ResolveInvoiceNumber(input string) string {

	cleaned := NormalizeInvoiceNumber(input)

	if cleaned == "" {
		return GenerateInvoiceNumber()
	}

	return cleaned
}

// InvoiceNumberExistsFn is the contract the application layer must
// satisfy in order to perform uniqueness checks.
//
// Defining this as a function type keeps the domain service free of
// any direct dependency on a database or repository implementation,
// which makes it easy to test and easy to swap.
type InvoiceNumberExistsFn func(ctx context.Context, number string) (bool, error)

// EnsureUniqueInvoiceNumber resolves an invoice number that is
// guaranteed not to collide with an existing record.
//
// Behavior:
//   - If the candidate is unused → return it unchanged
//   - If the candidate exists  → append a deterministic suffix and retry
//   - If conflicts persist past invoiceNumberMaxAttempts → return error
//
// The PostgreSQL UNIQUE constraint remains the final safety net,
// but it should rarely, if ever, be triggered if this function is used.
func EnsureUniqueInvoiceNumber(
	ctx context.Context,
	exists InvoiceNumberExistsFn,
	candidate string,
) (string, error) {

	if exists == nil {
		return "", fmt.Errorf("invoice number existence check is required")
	}

	current := candidate

	for attempt := 0; attempt < invoiceNumberMaxAttempts; attempt++ {

		taken, err := exists(ctx, current)
		if err != nil {
			return "", fmt.Errorf("invoice number lookup failed: %w", err)
		}

		if !taken {
			return current, nil
		}

		// Deterministic, time-based, attempt-aware suffix.
		// The attempt counter prevents loop-collisions when
		// many invoices are created within the same millisecond.
		suffix := time.Now().UnixNano()/int64(time.Millisecond) + int64(attempt)

		current = fmt.Sprintf("%s-%d", candidate, suffix)
	}

	return "", fmt.Errorf(
		"unable to resolve unique invoice number after %d attempts",
		invoiceNumberMaxAttempts,
	)
}
