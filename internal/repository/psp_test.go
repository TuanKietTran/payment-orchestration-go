package repository

import (
	"context"
	"testing"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/google/uuid"
)

func advanceCapture(t *testing.T, repo ProcessingRepository, id domain.PaymentID) {
	t.Helper()
	ctx := context.Background()
	auth, cap := uuid.NewString(), uuid.NewString()
	if err := repo.AdvancePSP(ctx, id, domain.StatusScreened, domain.StatusCaptured, auth); err == nil {
		t.Fatal("skipped authorization")
	}
	if err := repo.AdvancePSP(ctx, id, domain.StatusScreened, domain.StatusAuthorized, "invalid"); err == nil {
		t.Fatal("accepted invalid PSP reference")
	}
	if err := repo.AdvancePSP(ctx, id, domain.StatusScreened, domain.StatusAuthorized, auth); err != nil {
		t.Fatal(err)
	}
	if err := repo.AdvancePSP(ctx, id, domain.StatusScreened, domain.StatusAuthorized, auth); err != nil {
		t.Fatal(err)
	}
	if err := repo.AdvancePSP(ctx, id, domain.StatusScreened, domain.StatusAuthorized, uuid.NewString()); err == nil {
		t.Fatal("accepted conflicting authorization")
	}
	got, err := repo.GetByID(ctx, id)
	if err != nil || got.Status != domain.StatusAuthorized || got.AuthorizationID != auth {
		t.Fatalf("payment=%+v err=%v", got, err)
	}
	if err := repo.AdvancePSP(ctx, id, domain.StatusAuthorized, domain.StatusCaptured, cap); err != nil {
		t.Fatal(err)
	}
	if err := repo.AdvancePSP(ctx, id, domain.StatusAuthorized, domain.StatusCaptured, cap); err != nil {
		t.Fatal(err)
	}
	got, err = repo.GetByID(ctx, id)
	if err != nil || got.Status != domain.StatusCaptured || got.CaptureID != cap {
		t.Fatalf("payment=%+v err=%v", got, err)
	}
	if err := repo.DeclinePSP(ctx, id); err == nil {
		t.Fatal("declined after capture")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := repo.AdvancePSP(canceled, id, domain.StatusAuthorized, domain.StatusCaptured, cap); err == nil {
		t.Fatal("ignored cancellation")
	}
	if err := repo.DeclinePSP(canceled, id); err == nil {
		t.Fatal("ignored cancellation")
	}
}
