package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

var (
	_ PaymentRepository = (*PostgresPayment)(nil)
	_ PaymentRepository = (*InMemoryPayment)(nil)
)

// PostgresPayment persists payments against migrations/001_init.sql.
type PostgresPayment struct {
	pool *pgxpool.Pool
}

func NewPostgresPayment(ctx context.Context, dsn string) (*PostgresPayment, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}
	return &PostgresPayment{pool: pool}, nil
}

func (r *PostgresPayment) Close() {
	r.pool.Close()
}

func (r *PostgresPayment) Save(ctx context.Context, p domain.Payment) error {
	ledgerID, err := parseLedgerTxID(p.LedgerTxID)
	if err != nil {
		return err
	}

	var inserted uuid.UUID
	err = r.pool.QueryRow(ctx, `
		INSERT INTO payments (
			id, idempotency_key, source_account, dest_account,
			amount, currency, status, ledger_tx_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id
	`, p.ID, p.IdempotencyKey, p.SourceAccount, p.DestAccount, p.Amount.String(), p.Currency, string(p.Status), ledgerID).Scan(&inserted)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			var existing uuid.UUID
			if qerr := r.pool.QueryRow(ctx, `
				SELECT id FROM payments WHERE idempotency_key = $1
			`, p.IdempotencyKey).Scan(&existing); qerr != nil {
				return qerr
			}
			return fmt.Errorf("idempotency key already used: %s", existing)
		}
		return err
	}
	return nil
}

func (r *PostgresPayment) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Payment, error) {
	var id uuid.UUID
	if err := r.pool.QueryRow(ctx, `SELECT id FROM payments WHERE idempotency_key=$1`, key).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("payment not found")
		}
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *PostgresPayment) GetByID(ctx context.Context, id domain.PaymentID) (*domain.Payment, error) {
	var p domain.Payment
	var amountText, status string
	var ledgerID *uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT id, idempotency_key, source_account, dest_account,
		       amount::text, currency, status, ledger_tx_id
		FROM payments
		WHERE id = $1
	`, id).Scan(&p.ID, &p.IdempotencyKey, &p.SourceAccount, &p.DestAccount, &amountText, &p.Currency, &status, &ledgerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("payment not found")
		}
		return nil, err
	}
	amt, err := decimal.NewFromString(amountText)
	if err != nil {
		return nil, fmt.Errorf("parse amount: %w", err)
	}
	p.Amount = amt
	p.Status = domain.PaymentStatus(status)
	if ledgerID != nil {
		s := ledgerID.String()
		p.LedgerTxID = &s
	}
	return &p, nil
}

func (r *PostgresPayment) UpdateStatus(ctx context.Context, id domain.PaymentID, status domain.PaymentStatus) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE payments
		SET status = $2, updated_at = NOW()
		WHERE id = $1
	`, id, string(status))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("payment not found")
	}
	return nil
}

func parseLedgerTxID(raw *string) (*uuid.UUID, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(*raw)
	if err != nil {
		return nil, fmt.Errorf("ledger tx id: %w", err)
	}
	return &id, nil
}
