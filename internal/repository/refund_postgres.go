package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

var (
	_ RefundRepository = (*InMemoryPayment)(nil)
	_ RefundRepository = (*PostgresPayment)(nil)
)

const refundColumns = `id,payment_id,idempotency_key,amount::text,currency,status,ledger_tx_id`

func scanRefund(row interface{ Scan(...any) error }) (*domain.Refund, error) {
	var r domain.Refund
	var amount string
	var ref *uuid.UUID
	if err := row.Scan(&r.ID, &r.PaymentID, &r.IdempotencyKey, &amount, &r.Currency, &r.Status, &ref); err != nil {
		return nil, err
	}
	var err error
	r.Amount, err = decimal.NewFromString(amount)
	if err != nil {
		return nil, err
	}
	if ref != nil {
		s := ref.String()
		r.LedgerTxID = &s
	}
	return &r, nil
}
func (r *PostgresPayment) ReserveRefund(ctx context.Context, id domain.PaymentID, key string, amount decimal.Decimal) (*domain.Refund, error) {
	if err := validRefundRequest(key, amount); err != nil {
		return nil, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var amountText, currency string
	var status domain.PaymentStatus
	if err := tx.QueryRow(ctx, `SELECT amount::text,currency,status FROM payments WHERE id=$1 FOR UPDATE`, id).Scan(&amountText, &currency, &status); err != nil {
		return nil, err
	}
	existing, err := scanRefund(tx.QueryRow(ctx, `SELECT `+refundColumns+` FROM payment_refunds WHERE idempotency_key=$1`, key))
	if err == nil {
		if err := sameRefund(*existing, id, amount); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if !domain.CanRefund(status) {
		return nil, domain.ErrInvalidRefund
	}
	paymentAmount, err := decimal.NewFromString(amountText)
	if err != nil {
		return nil, err
	}
	var totalText string
	if err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(amount),0)::text FROM payment_refunds WHERE payment_id=$1`, id).Scan(&totalText); err != nil {
		return nil, err
	}
	total, err := decimal.NewFromString(totalText)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateRefund(paymentAmount, total, amount); err != nil {
		return nil, err
	}
	refund, err := scanRefund(tx.QueryRow(ctx, `INSERT INTO payment_refunds(id,payment_id,idempotency_key,amount,currency,status) VALUES($1,$2,$3,$4,$5,'PENDING') ON CONFLICT(idempotency_key) DO NOTHING RETURNING `+refundColumns, uuid.New(), id, key, amount.String(), currency))
	if errors.Is(err, pgx.ErrNoRows) {
		refund, err = scanRefund(tx.QueryRow(ctx, `SELECT `+refundColumns+` FROM payment_refunds WHERE idempotency_key=$1`, key))
		if err == nil {
			err = sameRefund(*refund, id, amount)
		}
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return refund, nil
}
func (r *PostgresPayment) CompleteRefund(ctx context.Context, id uuid.UUID, ref string) (*domain.Refund, error) {
	ledgerID, err := uuid.Parse(ref)
	if err != nil {
		return nil, fmt.Errorf("ledger tx id: %w", err)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var paymentID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT payment_id FROM payment_refunds WHERE id=$1`, id).Scan(&paymentID); err != nil {
		return nil, err
	}
	var amountText, totalText string
	if err := tx.QueryRow(ctx, `SELECT amount::text,refunded_amount::text FROM payments WHERE id=$1 FOR UPDATE`, paymentID).Scan(&amountText, &totalText); err != nil {
		return nil, err
	}
	refund, err := scanRefund(tx.QueryRow(ctx, `SELECT `+refundColumns+` FROM payment_refunds WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return nil, err
	}
	if refund.Status == domain.StatusSettled {
		if refund.LedgerTxID == nil || *refund.LedgerTxID != ref {
			return nil, domain.ErrRefundConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return refund, nil
	}
	amount, err := decimal.NewFromString(amountText)
	if err != nil {
		return nil, err
	}
	total, err := decimal.NewFromString(totalText)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateRefund(amount, total, refund.Amount); err != nil {
		return nil, err
	}
	total = total.Add(refund.Amount)
	if _, err := tx.Exec(ctx, `UPDATE payments SET refunded_amount=$2,status=$3,updated_at=NOW() WHERE id=$1`, paymentID, total.String(), string(domain.RefundStatus(total, amount))); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE payment_refunds SET status='SETTLED',ledger_tx_id=$2 WHERE id=$1`, id, ledgerID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	refund.Status, refund.LedgerTxID = domain.StatusSettled, &ref
	return refund, nil
}
func (r *PostgresPayment) PendingRefunds(ctx context.Context, limit int) ([]domain.Refund, error) {
	if limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("limit must be between 1 and 1000")
	}
	rows, err := r.pool.Query(ctx, `SELECT `+refundColumns+` FROM payment_refunds WHERE status='PENDING' ORDER BY id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	pending := make([]domain.Refund, 0)
	for rows.Next() {
		refund, err := scanRefund(rows)
		if err != nil {
			return nil, err
		}
		pending = append(pending, *refund)
	}
	return pending, rows.Err()
}
