package database

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

// WithTenantRLS runs fn inside a transaction with app.tenant_id set for RLS policies.
func WithTenantRLS(ctx context.Context, tenantID uuid.UUID, fn func(tx *sql.Tx) error) error {
	tx, err := PostgresDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantID.String()); err != nil {
		return err
	}
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
