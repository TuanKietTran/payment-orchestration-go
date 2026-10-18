package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestPostgresPayment(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("embedded Postgres cannot initdb as root")
	}
	dsn := startPostgres(t, "payments", 55433)
	ctx := context.Background()

	repo, err := NewPostgresPayment(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(repo.Close)

	sqlBytes, err := os.ReadFile(filepath.Join("..", "..", "migrations", "001_init.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.pool.Exec(ctx, string(sqlBytes)); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	outboxSQL, err := os.ReadFile(filepath.Join("..", "..", "migrations", "002_processing_outbox.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.pool.Exec(ctx, string(outboxSQL)); err != nil {
		t.Fatal(err)
	}
	refundSQL, err := os.ReadFile(filepath.Join("..", "..", "migrations", "003_refunds.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.pool.Exec(ctx, string(refundSQL)); err != nil {
		t.Fatal(err)
	}
	legacy := domain.Payment{ID: uuid.New(), IdempotencyKey: uuid.NewString(), SourceAccount: "legacy-source", DestAccount: "legacy-dest", Amount: decimal.NewFromInt(1), Currency: "USD", Status: domain.StatusProcessing}
	if err := repo.Save(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.pool.Exec(ctx, `INSERT INTO payment_outbox(payment_id) VALUES($1)`, legacy.ID); err != nil {
		t.Fatal(err)
	}
	pspSQL, err := os.ReadFile(filepath.Join("..", "..", "migrations", "004_psp_progress.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.pool.Exec(ctx, string(pspSQL)); err != nil {
		t.Fatal(err)
	}
	migrated, err := repo.GetByID(ctx, legacy.ID)
	if err != nil || migrated.Status != domain.StatusScreened || migrated.FraudDecision != "ALLOW" {
		t.Fatalf("legacy migration=%+v err=%v", migrated, err)
	}
	advanceCapture(t, repo, legacy.ID)
	if err := repo.CompletePosting(ctx, legacy.ID, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	processingContract(t, repo)
	refundContract(t, repo)
	listContract(t, repo)
	if err := repo.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	// Persist an unfinished intent and recover from a fresh independent pool.
	pending := domain.Payment{ID: uuid.New(), IdempotencyKey: uuid.NewString(), SourceAccount: "source", DestAccount: "dest", Amount: decimal.NewFromInt(1), Currency: "USD", Status: domain.StatusPending}
	if err := repo.Save(ctx, pending); err != nil {
		t.Fatal(err)
	}
	if err := repo.StageDecision(ctx, pending.ID, domain.StatusProcessing); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewPostgresPayment(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	ids, err := reopened.PendingPostings(ctx, 100)
	if err != nil || len(ids) != 1 || ids[0] != pending.ID {
		t.Fatalf("reopened intents=%v err=%v", ids, err)
	}
	// An independent pool also sees pending refund intents across adapter lifetimes.
	refundPayment := domain.Payment{ID: uuid.New(), IdempotencyKey: uuid.NewString(), SourceAccount: "source", DestAccount: "dest", Amount: decimal.NewFromInt(2), Currency: "USD", Status: domain.StatusSettled}
	if err := repo.Save(ctx, refundPayment); err != nil {
		t.Fatal(err)
	}
	reserved, err := repo.ReserveRefund(ctx, refundPayment.ID, "reopen-refund", decimal.NewFromInt(1))
	if err != nil {
		t.Fatal(err)
	}
	pendingRefunds, err := reopened.PendingRefunds(ctx, 100)
	if err != nil || len(pendingRefunds) != 1 || pendingRefunds[0].ID != reserved.ID {
		t.Fatalf("reopened refunds=%v err=%v", pendingRefunds, err)
	}
	if _, err := reopened.CompleteRefund(ctx, reserved.ID, uuid.NewString()); err != nil {
		t.Fatal(err)
	}

	advanceCapture(t, reopened, pending.ID)
	ref := uuid.NewString()
	if err := reopened.CompletePosting(ctx, pending.ID, ref); err != nil {
		t.Fatal(err)
	}
	recovered, err := repo.GetByID(ctx, pending.ID)
	if err != nil || recovered.LedgerTxID == nil || *recovered.LedgerTxID != ref || recovered.AuthorizationID == "" || recovered.CaptureID == "" {
		t.Fatalf("recovered=%+v err=%v", recovered, err)
	}

	ledger := uuid.New().String()
	p := domain.Payment{
		ID:             uuid.New(),
		IdempotencyKey: "pay-pg-1",
		SourceAccount:  "acct-src",
		DestAccount:    "acct-dst",
		Amount:         decimal.RequireFromString("19.99"),
		Currency:       "USD",
		Status:         domain.StatusPending,
		LedgerTxID:     &ledger,
	}
	if err := repo.Save(ctx, p); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SourceAccount != "acct-src" || got.DestAccount != "acct-dst" || got.Currency != "USD" {
		t.Fatalf("unexpected payment: %+v", got)
	}
	if !got.Amount.Equal(decimal.RequireFromString("19.99")) || got.Status != domain.StatusPending {
		t.Fatalf("amount/status = %s %s", got.Amount, got.Status)
	}
	if got.LedgerTxID == nil || *got.LedgerTxID != ledger {
		t.Fatalf("ledger tx = %v", got.LedgerTxID)
	}

	dup := p
	dup.ID = uuid.New()
	err = repo.Save(ctx, dup)
	if err == nil || !strings.Contains(err.Error(), "idempotency key already used") || !strings.Contains(err.Error(), p.ID.String()) {
		t.Fatalf("err = %v", err)
	}

	if err := repo.UpdateStatus(ctx, p.ID, domain.StatusSucceeded); err != nil {
		t.Fatal(err)
	}
	got, err = repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.StatusSucceeded {
		t.Fatalf("status = %s", got.Status)
	}

	if _, err := repo.GetByID(ctx, uuid.New()); err == nil || !strings.Contains(err.Error(), "payment not found") {
		t.Fatalf("get missing err = %v", err)
	}
	if err := repo.UpdateStatus(ctx, uuid.New(), domain.StatusFailed); err == nil || !strings.Contains(err.Error(), "payment not found") {
		t.Fatalf("update missing err = %v", err)
	}
}

func startPostgres(t *testing.T, dbName string, port uint32) string {
	t.Helper()
	cfg := embeddedpostgres.DefaultConfig().
		Username("postgres").
		Password("postgres").
		Database(dbName).
		Port(port).
		Version(embeddedpostgres.V16).
		CachePath("/tmp/embedded-pg-cache").
		RuntimePath(filepath.Join(t.TempDir(), "rt")).
		DataPath(filepath.Join(t.TempDir(), "data")).
		StartTimeout(90 * time.Second).
		Logger(os.Stdout)
	ep := embeddedpostgres.NewDatabase(cfg)
	if err := ep.Start(); err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = ep.Stop() })
	return fmt.Sprintf("postgres://postgres:postgres@127.0.0.1:%d/%s?sslmode=disable", port, dbName)
}
