// internal/repo/audit_repo.go
package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"
)

// AuditRepo writes audit log entries to the database.
// This repo is append-only — no update or delete methods exist by design.
type AuditRepo struct{ db *sql.DB }

func NewAuditRepo(db *sql.DB) *AuditRepo { return &AuditRepo{db: db} }


// Log writes a single audit entry. Errors are logged but never surfaced
// to the caller — audit logging must never block or fail a user action.
func (r *AuditRepo) Log(ctx context.Context, entry AuditLog) {
	if r.db == nil {
		return
	}

	var metadataJSON []byte
	if entry.Metadata != nil {
		b, err := json.Marshal(entry.Metadata)
		if err != nil {
			slog.Error("audit metadata marshal failed", "action", entry.Action, "err", err)
		} else {
			metadataJSON = b
		}
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO audit_logs
			(user_id, action, entity_type, entity_id, ip_address, metadata, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NOW())`,
		entry.UserID,
		entry.Action,
		entry.EntityType,
		entry.EntityID,
		entry.IPAddress,
		metadataJSON,
	)
	if err != nil {
		slog.Error("audit log write failed", "action", entry.Action, "entity_type", entry.EntityType, "entity_id", entry.EntityID, "err", err)
	}
}
// ListByUser returns the most recent audit entries for a given user.
// Used for user-facing activity history and admin review.
func (r *AuditRepo) ListByUser(ctx context.Context, userID int64, limit int) ([]AuditLog, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, action, entity_type, entity_id,
		        ip_address, metadata, created_at
		 FROM audit_logs
		 WHERE user_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAuditLogs(rows)
}

// ListByEntity returns all audit entries for a specific record.
// Example: all actions taken on invoice #42.
func (r *AuditRepo) ListByEntity(ctx context.Context, entityType string, entityID int64) ([]AuditLog, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, action, entity_type, entity_id,
		        ip_address, metadata, created_at
		 FROM audit_logs
		 WHERE entity_type = $1 AND entity_id = $2
		 ORDER BY created_at DESC`,
		entityType, entityID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAuditLogs(rows)
}

// ListRecent returns the most recent audit entries across all users.
// Used for admin security review and anomaly detection.
func (r *AuditRepo) ListRecent(ctx context.Context, limit int) ([]AuditLog, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, action, entity_type, entity_id,
		        ip_address, metadata, created_at
		 FROM audit_logs
		 ORDER BY created_at DESC
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAuditLogs(rows)
}

// scanAuditLogs maps database rows into AuditLog structs.
func scanAuditLogs(rows *sql.Rows) ([]AuditLog, error) {
	var logs []AuditLog

	for rows.Next() {
		var entry AuditLog
		var metadataRaw []byte
		var entityID sql.NullInt64
		var userID sql.NullInt64
		var ipAddress sql.NullString
		var createdAt time.Time

		if err := rows.Scan(
			&entry.ID,
			&userID,
			&entry.Action,
			&entry.EntityType,
			&entityID,
			&ipAddress,
			&metadataRaw,
			&createdAt,
		); err != nil {
			return nil, err
		}

		if userID.Valid {
			entry.UserID = &userID.Int64
		}
		if entityID.Valid {
			entry.EntityID = &entityID.Int64
		}
		entry.IPAddress = ipAddress.String
		entry.CreatedAt = createdAt

		if metadataRaw != nil {
			if err := json.Unmarshal(metadataRaw, &entry.Metadata); err != nil {
				slog.Warn("audit metadata unmarshal failed", "log_id", entry.ID, "err", err)
			}
		}

		logs = append(logs, entry)
	}

	return logs, rows.Err()
}
// ListRecentFiltered returns paginated audit entries with optional action filter.
// Used by the admin audit viewer at /admin/audit.
// Pass action="" to return all actions.
func (r *AuditRepo) ListRecentFiltered(ctx context.Context, action string, limit, offset int) ([]AuditLog, error) {
	var rows *sql.Rows
	var err error

	if action == "" {
		rows, err = r.db.QueryContext(ctx,
			`SELECT id, user_id, action, entity_type, entity_id,
			        ip_address, metadata, created_at
			 FROM audit_logs
			 ORDER BY created_at DESC
			 LIMIT $1 OFFSET $2`,
			limit, offset,
		)
	} else {
		rows, err = r.db.QueryContext(ctx,
			`SELECT id, user_id, action, entity_type, entity_id,
			        ip_address, metadata, created_at
			 FROM audit_logs
			 WHERE action = $1
			 ORDER BY created_at DESC
			 LIMIT $2 OFFSET $3`,
			action, limit, offset,
		)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAuditLogs(rows)
}

// CountFiltered returns the total count of audit entries for pagination math.
// Pass action="" to count all entries.
func (r *AuditRepo) CountFiltered(ctx context.Context, action string) (int, error) {
	var count int
	var err error

	if action == "" {
		err = r.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM audit_logs`,
		).Scan(&count)
	} else {
		err = r.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM audit_logs WHERE action = $1`,
			action,
		).Scan(&count)
	}

	return count, err
}

