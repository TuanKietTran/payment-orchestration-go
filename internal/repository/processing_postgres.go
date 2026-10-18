package repository

import (
	"context"
	"fmt"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/google/uuid"
)

var (
	_ ProcessingRepository = (*PostgresPayment)(nil)
	_ ProcessingRepository = (*InMemoryPayment)(nil)
)

func (r *PostgresPayment) StageDecision(ctx context.Context, id domain.PaymentID, status domain.PaymentStatus) error {
	if !validDecisionStatus(status) {
		return fmt.Errorf("invalid decision status %q", status)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var current domain.PaymentStatus
	if err := tx.QueryRow(ctx, `SELECT status FROM payments WHERE id=$1 FOR UPDATE`, id).Scan(&current); err != nil {
		return err
	}
	if current != domain.StatusPending && current != domain.StatusCreated {
		return tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `UPDATE payments SET status=$2, updated_at=NOW() WHERE id=$1`, id, string(status)); err != nil {
		return err
	}
	if status == domain.StatusProcessing {
		if _, err := tx.Exec(ctx, `INSERT INTO payment_outbox(payment_id) VALUES($1)`, id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *PostgresPayment) CompletePosting(ctx context.Context, id domain.PaymentID, ref string) error {
	ledgerID, err := uuid.Parse(ref)
	if err != nil {
		return fmt.Errorf("ledger tx id: %w", err)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var status domain.PaymentStatus
	var existing *uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT status, ledger_tx_id FROM payments WHERE id=$1 FOR UPDATE`, id).Scan(&status, &existing); err != nil {
		return err
	}
	if status == domain.StatusSettled && existing != nil && *existing == ledgerID {
		return tx.Commit(ctx)
	}
	if status != domain.StatusProcessing {
		return fmt.Errorf("posting intent missing or conflicting")
	}
	tag, err := tx.Exec(ctx, `DELETE FROM payment_outbox WHERE payment_id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("posting intent missing or conflicting")
	}
	if _, err := tx.Exec(ctx, `UPDATE payments SET status=$2, ledger_tx_id=$3, updated_at=NOW() WHERE id=$1`, id, string(domain.StatusSettled), ledgerID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresPayment) PendingPostings(ctx context.Context, limit int) ([]domain.PaymentID, error) {
	if limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("limit must be between 1 and 1000")
	}
	rows, err := r.pool.Query(ctx, `SELECT payment_id FROM payment_outbox ORDER BY payment_id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]domain.PaymentID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
