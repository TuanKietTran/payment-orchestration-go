package command

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/TuanKietTran/payment-orchestration-go/internal/downstream"
	"github.com/TuanKietTran/payment-orchestration-go/internal/repository"
	"github.com/shopspring/decimal"
)

type completionFault struct {
	repository.ProcessingRepository
	fail bool
}

func (r *completionFault) CompletePosting(ctx context.Context, id domain.PaymentID, ref string) error {
	if r.fail {
		r.fail = false
		return errors.New("injected database completion failure")
	}
	return r.ProcessingRepository.CompletePosting(ctx, id, ref)
}

type ambiguousLedger struct {
	*downstream.SimulatedLedger
	fail bool
}

func (l *ambiguousLedger) Post(ctx context.Context, key, source, dest string, amount decimal.Decimal, currency string) (string, error) {
	ref, err := l.SimulatedLedger.Post(ctx, key, source, dest, amount, currency)
	if l.fail {
		l.fail = false
		return "", errors.New("injected response lost after commit")
	}
	return ref, err
}

type rejectFraud struct{ t *testing.T }

func (f rejectFraud) Score(context.Context, string, string, decimal.Decimal, string) (downstream.FraudDecision, error) {
	f.t.Error("recovery rescored an already committed decision")
	return downstream.FraudDecision{}, errors.New("unexpected rescore")
}

func TestRecoveryReplaysAmbiguousPostingAndCompletionFailure(t *testing.T) {
	for _, failure := range []string{"ledger response lost", "database completion failed"} {
		t.Run(failure, func(t *testing.T) {
			ctx := context.Background()
			memory := repository.NewInMemoryPayment()
			repo := &completionFault{ProcessingRepository: memory, fail: failure == "database completion failed"}
			ledger := &ambiguousLedger{SimulatedLedger: downstream.NewSimulatedLedger(), fail: failure == "ledger response lost"}
			id, err := NewCreatePaymentHandler(repo).Handle(ctx, CreatePaymentCommand{IdempotencyKey: "recover", SourceAccount: "source", DestAccount: "dest", Amount: decimal.NewFromInt(10), Currency: "USD"})
			if err != nil {
				t.Fatal(err)
			}
			h := NewProcessPaymentHandler(repo, downstream.SimulatedFraud{}, ledger)
			if _, err = h.Handle(ctx, id); err == nil {
				t.Fatal("fault not observed")
			}
			p, err := repo.GetByID(ctx, id)
			if err != nil || p.Status != domain.StatusCaptured || p.AuthorizationID == "" || p.CaptureID == "" || p.LedgerTxID != nil {
				t.Fatalf("false success %+v err=%v", p, err)
			}
			pending, err := repo.PendingPostings(ctx, 10)
			if err != nil || len(pending) != 1 {
				t.Fatalf("lost intent: %v %v", pending, err)
			}
			// A fresh handler has no in-process progress state.
			restarted := NewProcessPaymentHandler(memory, rejectFraud{t}, ledger)
			if err := restarted.Recover(ctx, 100); err != nil {
				t.Fatal(err)
			}
			p, err = memory.GetByID(ctx, id)
			if err != nil || p.Status != domain.StatusSettled || p.LedgerTxID == nil {
				t.Fatalf("not recovered %+v err=%v", p, err)
			}
			if len(ledger.Transactions) != 1 || ledger.Transactions["payment:recover"] != *p.LedgerTxID {
				t.Fatalf("posting mismatch: %v %+v", ledger.Transactions, p)
			}
			if err := restarted.Recover(ctx, 100); err != nil {
				t.Fatal(err)
			}
			if len(ledger.Transactions) != 1 {
				t.Fatal("duplicate posting")
			}
		})
	}
}

func TestConcurrentProcessAndRecoveryDoNotDuplicatePosting(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewInMemoryPayment()
	ledger := downstream.NewSimulatedLedger()
	id, err := NewCreatePaymentHandler(repo).Handle(ctx, CreatePaymentCommand{IdempotencyKey: "concurrent", SourceAccount: "source", DestAccount: "dest", Amount: decimal.NewFromInt(10), Currency: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	h := NewProcessPaymentHandler(repo, downstream.SimulatedFraud{}, ledger)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := h.Handle(ctx, id); err != nil {
				t.Error(err)
			}
			if err := h.Recover(ctx, 10); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	p, err := repo.GetByID(ctx, id)
	if err != nil || p.Status != domain.StatusSettled || p.LedgerTxID == nil || len(ledger.Transactions) != 1 {
		t.Fatalf("payment=%+v transactions=%v err=%v", p, ledger.Transactions, err)
	}
}
