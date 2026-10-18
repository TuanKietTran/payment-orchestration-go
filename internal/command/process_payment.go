package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/TuanKietTran/payment-orchestration-go/internal/downstream"
	"github.com/TuanKietTran/payment-orchestration-go/internal/repository"
)

type ProcessPaymentHandler struct {
	repo   repository.ProcessingRepository
	fraud  downstream.Fraud
	ledger downstream.Ledger
	psp    downstream.PSP
}

func NewProcessPaymentHandler(r repository.ProcessingRepository, f downstream.Fraud, l downstream.Ledger, adapters ...downstream.PSP) *ProcessPaymentHandler {
	var psp downstream.PSP = downstream.NewSimulatedPSP(nil)
	if len(adapters) > 0 {
		psp = adapters[0]
	}
	return &ProcessPaymentHandler{repo: r, fraud: f, ledger: l, psp: downstream.NewResilientPSP(psp)}
}
func (h *ProcessPaymentHandler) Handle(ctx context.Context, id domain.PaymentID) (*domain.Payment, error) {
	p, err := h.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Status == domain.StatusPending || p.Status == domain.StatusCreated {
		d, err := h.fraud.Score(ctx, p.IdempotencyKey, p.SourceAccount, p.Amount, p.Currency)
		if err != nil {
			return nil, err
		}
		var status domain.PaymentStatus
		switch d.Decision {
		case "DENY", "BLOCK":
			status = domain.StatusFailed
		case "REVIEW":
			status = domain.StatusScreened
		case "ALLOW":
			status = domain.StatusProcessing
		default:
			return nil, fmt.Errorf("unknown fraud decision %q", d.Decision)
		}
		// A concurrent caller cannot overwrite a committed decision. ALLOW and its
		// posting intent become visible atomically, before any external money move.
		if err := h.repo.StageDecision(ctx, id, status); err != nil {
			return nil, err
		}
		p, err = h.repo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
	}
	for p.Status == domain.StatusAuthorized || (p.Status == domain.StatusScreened && p.FraudDecision == "ALLOW") {
		from := p.Status
		to := domain.StatusAuthorized
		var ref string
		if from == domain.StatusScreened {
			ref, err = h.psp.Authorize(ctx, "authorize:"+p.IdempotencyKey, p.Amount, p.Currency)
		} else {
			to = domain.StatusCaptured
			ref, err = h.psp.Capture(ctx, "capture:"+p.IdempotencyKey, p.AuthorizationID, p.Amount, p.Currency)
		}
		if errors.Is(err, downstream.ErrPSPDeclined) {
			if err := h.repo.DeclinePSP(ctx, id); err != nil {
				return nil, err
			}
			return h.repo.GetByID(ctx, id)
		}
		if err != nil {
			return nil, err
		}
		if err := h.repo.AdvancePSP(ctx, id, from, to, ref); err != nil {
			return nil, err
		}
		p, err = h.repo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
	}
	if p.Status != domain.StatusCaptured {
		return p, nil
	}
	// All dispatchers use the same downstream key. A crash or ambiguous timeout
	// after ledger commit is recovered by replay, not by posting a fresh transfer.
	ref, err := h.ledger.Post(ctx, "payment:"+p.IdempotencyKey, p.SourceAccount, p.DestAccount, p.Amount, p.Currency)
	if err != nil {
		return nil, err
	}
	if err := h.repo.CompletePosting(ctx, id, ref); err != nil {
		return nil, err
	}
	return h.repo.GetByID(ctx, id)
}

// Recover attempts one bounded batch and continues past individual failures.
// Multiple workers may dispatch the same intent: the ledger must deduplicate.
func (h *ProcessPaymentHandler) Recover(ctx context.Context, limit int) error {
	ids, err := h.repo.PendingPostings(ctx, limit)
	if err != nil {
		return err
	}
	var failures []error
	for _, id := range ids {
		if _, err := h.Handle(ctx, id); err != nil {
			failures = append(failures, fmt.Errorf("recover payment %s: %w", id, err))
		}
	}
	return errors.Join(failures...)
}
