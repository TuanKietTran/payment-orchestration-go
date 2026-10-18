package repository

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
)

// PaymentLister pages payments in ascending ID order. after==uuid.Nil starts at
// the beginning; the cursor is exclusive.
type PaymentLister interface {
	List(ctx context.Context, after uuid.UUID, limit int) ([]domain.Payment, error)
}

func (r *InMemoryPayment) List(ctx context.Context, after uuid.UUID, limit int) ([]domain.Payment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]domain.Payment, 0)
	for _, p := range r.payments {
		if after == uuid.Nil || p.ID.String() > after.String() {
			out = append(out, clonePayment(p))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID.String() < out[j].ID.String() })
	if limit >= 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *PostgresPayment) List(ctx context.Context, after uuid.UUID, limit int) ([]domain.Payment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id FROM payments
		WHERE $1::uuid = '00000000-0000-0000-0000-000000000000'::uuid OR id > $1::uuid
		ORDER BY id
		LIMIT $2
	`, after, limit)
	if err != nil {
		return nil, err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]domain.Payment, 0, len(ids))
	for _, id := range ids {
		p, err := r.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, nil
}
