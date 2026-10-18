package command

import (
	"context"
	"errors"
	"testing"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/TuanKietTran/payment-orchestration-go/internal/downstream"
	"github.com/TuanKietTran/payment-orchestration-go/internal/repository"
	"github.com/shopspring/decimal"
)

type pspProgressFault struct {
	repository.ProcessingRepository
	fail domain.PaymentStatus
}

func (r *pspProgressFault) AdvancePSP(ctx context.Context, id domain.PaymentID, from, to domain.PaymentStatus, ref string) error {
	if r.fail == to {
		r.fail = ""
		return errors.New("injected PSP progress persistence failure")
	}
	return r.ProcessingRepository.AdvancePSP(ctx, id, from, to, ref)
}
func TestPSPProgressRecoversAfterLocalCommitFailure(t *testing.T) {
	for _, step := range []domain.PaymentStatus{domain.StatusAuthorized, domain.StatusCaptured} {
		t.Run(string(step), func(t *testing.T) {
			ctx := context.Background()
			memory := repository.NewInMemoryPayment()
			ledger := downstream.NewSimulatedLedger()
			commits := map[string]int{}
			psp := downstream.NewSimulatedPSP(func(op, key string, attempt int, after bool) error {
				if after {
					commits[op]++
				}
				return nil
			})
			id, err := NewCreatePaymentHandler(memory).Handle(ctx, CreatePaymentCommand{IdempotencyKey: "psp-progress", SourceAccount: "source", DestAccount: "dest", Amount: decimal.NewFromInt(10), Currency: "USD"})
			if err != nil {
				t.Fatal(err)
			}
			faulty := &pspProgressFault{ProcessingRepository: memory, fail: step}
			if _, err := NewProcessPaymentHandler(faulty, downstream.SimulatedFraud{}, ledger, psp).Handle(ctx, id); err == nil {
				t.Fatal("fault not observed")
			}
			p, err := memory.GetByID(ctx, id)
			want := domain.StatusScreened
			if step == domain.StatusCaptured {
				want = domain.StatusAuthorized
			}
			if err != nil || p.Status != want || len(ledger.Transactions) != 0 {
				t.Fatalf("payment=%+v err=%v ledger=%v", p, err, ledger.Transactions)
			}
			restarted := NewProcessPaymentHandler(memory, rejectFraud{t}, ledger, psp)
			if err := restarted.Recover(ctx, 100); err != nil {
				t.Fatal(err)
			}
			p, err = memory.GetByID(ctx, id)
			if err != nil || p.Status != domain.StatusSettled || p.AuthorizationID == "" || p.CaptureID == "" || len(ledger.Transactions) != 1 {
				t.Fatalf("payment=%+v err=%v", p, err)
			}
			if commits["authorize"] != 1 || commits["capture"] != 1 {
				t.Fatalf("duplicate PSP effect: %v", commits)
			}
		})
	}
}
func TestPSPDeclineNeverPostsCash(t *testing.T) {
	for _, operation := range []string{"authorize", "capture"} {
		t.Run(operation, func(t *testing.T) {
			ctx := context.Background()
			repo := repository.NewInMemoryPayment()
			ledger := downstream.NewSimulatedLedger()
			psp := downstream.NewSimulatedPSP(func(op, key string, attempt int, after bool) error {
				if op == operation && !after {
					return downstream.ErrPSPDeclined
				}
				return nil
			})
			id, err := NewCreatePaymentHandler(repo).Handle(ctx, CreatePaymentCommand{IdempotencyKey: "decline", SourceAccount: "source", DestAccount: "dest", Amount: decimal.NewFromInt(10), Currency: "USD"})
			if err != nil {
				t.Fatal(err)
			}
			h := NewProcessPaymentHandler(repo, downstream.SimulatedFraud{}, ledger, psp)
			p, err := h.Handle(ctx, id)
			if err != nil || p.Status != domain.StatusFailed || len(ledger.Transactions) != 0 {
				t.Fatalf("payment=%+v err=%v", p, err)
			}
			pending, err := repo.PendingPostings(ctx, 10)
			if err != nil || len(pending) != 0 {
				t.Fatalf("pending=%v err=%v", pending, err)
			}
			if _, err := h.Handle(ctx, id); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestPSPCaptureTransientFailureRetainsAuthorizationForRecovery(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewInMemoryPayment()
	ledger := downstream.NewSimulatedLedger()
	unavailable := true
	psp := downstream.NewSimulatedPSP(func(op, key string, attempt int, after bool) error {
		if op == "capture" && unavailable {
			return downstream.ErrPSPTransient
		}
		return nil
	})
	id, err := NewCreatePaymentHandler(repo).Handle(ctx, CreatePaymentCommand{IdempotencyKey: "capture-recovery", SourceAccount: "source", DestAccount: "dest", Amount: decimal.NewFromInt(10), Currency: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	h := NewProcessPaymentHandler(repo, downstream.SimulatedFraud{}, ledger, psp)
	if _, err := h.Handle(ctx, id); !errors.Is(err, downstream.ErrPSPTransient) {
		t.Fatal(err)
	}
	p, err := repo.GetByID(ctx, id)
	if err != nil || p.Status != domain.StatusAuthorized || p.AuthorizationID == "" || len(ledger.Transactions) != 0 {
		t.Fatalf("payment=%+v err=%v", p, err)
	}
	unavailable = false
	if err := NewProcessPaymentHandler(repo, rejectFraud{t}, ledger, psp).Recover(ctx, 100); err != nil {
		t.Fatal(err)
	}
	p, err = repo.GetByID(ctx, id)
	if err != nil || p.Status != domain.StatusSettled || len(ledger.Transactions) != 1 {
		t.Fatalf("payment=%+v err=%v", p, err)
	}
}
