package repository

import (
	"context"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
)

func (r *PostgresPayment) AdvancePSP(ctx context.Context, id domain.PaymentID, from, to domain.PaymentStatus, ref string) error {
	if err := validPSPStep(from, to, ref); err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	p := domain.Payment{ID: id}
	if err := tx.QueryRow(ctx, `SELECT status,fraud_decision,authorization_id,capture_id FROM payments WHERE id=$1 FOR UPDATE`, id).Scan(&p.Status, &p.FraudDecision, &p.AuthorizationID, &p.CaptureID); err != nil {
		return err
	}
	updated, err := applyPSPStep(p, from, to, ref)
	if err != nil {
		return err
	}
	if updated.Status == p.Status {
		return tx.Commit(ctx)
	}
	var intent bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM payment_outbox WHERE payment_id=$1)`, id).Scan(&intent); err != nil {
		return err
	}
	if !intent {
		return domain.ErrIllegalTransition
	}
	if _, err := tx.Exec(ctx, `UPDATE payments SET status=$2,authorization_id=$3,capture_id=$4,updated_at=NOW() WHERE id=$1`, id, string(updated.Status), updated.AuthorizationID, updated.CaptureID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r *PostgresPayment) DeclinePSP(ctx context.Context, id domain.PaymentID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var status domain.PaymentStatus
	if err := tx.QueryRow(ctx, `SELECT status FROM payments WHERE id=$1 FOR UPDATE`, id).Scan(&status); err != nil {
		return err
	}
	if status == domain.StatusFailed {
		return tx.Commit(ctx)
	}
	if status != domain.StatusScreened && status != domain.StatusAuthorized {
		return domain.ErrIllegalTransition
	}
	tag, err := tx.Exec(ctx, `DELETE FROM payment_outbox WHERE payment_id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrIllegalTransition
	}
	if _, err := tx.Exec(ctx, `UPDATE payments SET status=$2,updated_at=NOW() WHERE id=$1`, id, string(domain.StatusFailed)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
