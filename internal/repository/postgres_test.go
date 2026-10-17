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
