package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/TuanKietTran/payment-orchestration-go/internal/api"
	"github.com/TuanKietTran/payment-orchestration-go/internal/command"
	"github.com/TuanKietTran/payment-orchestration-go/internal/downstream"
	"github.com/TuanKietTran/payment-orchestration-go/internal/query"
	"github.com/TuanKietTran/payment-orchestration-go/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	repo, cleanup := openRepo(ctx)
	defer cleanup()

	createH := command.NewCreatePaymentHandler(repo)
	getH := query.NewGetPaymentHandler(repo)
	var fraud downstream.Fraud = downstream.SimulatedFraud{}
	var ledger downstream.Ledger = downstream.NewSimulatedLedger()
	if base := os.Getenv("FRAUD_URL"); base != "" {
		client, err := downstream.NewHTTP(base)
		if err != nil {
			log.Fatal(err)
		}
		fraud = client
	}
	if base := os.Getenv("LEDGER_URL"); base != "" {
		client, err := downstream.NewHTTP(base)
		if err != nil {
			log.Fatal(err)
		}
		ledger = client
	}
	processH := command.NewProcessPaymentHandler(repo, fraud, ledger)
	refundH := command.NewRefundPaymentHandler(repo, ledger)
	handlers := api.NewHandlers(createH, getH, processH).WithRefund(refundH)
	if lister, ok := repo.(repository.PaymentLister); ok {
		if reader, ok := ledger.(downstream.LedgerReader); ok {
			handlers.WithListing(lister, query.NewReconcileHandler(lister, reader))
		}
	}
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			if err := processH.Recover(ctx, 100); err != nil && ctx.Err() == nil {
				log.Printf("outbox recovery: %v", err)
			}
			if err := refundH.Recover(ctx, 100); err != nil && ctx.Err() == nil {
				log.Printf("refund recovery: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	pinger, _ := repo.(api.Pinger)
	api.MountOps(r, pinger)
	r.Mount("/v1", handlers.Routes())

	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8102"
	}
	log.Printf("Payment API listening on %s", addr)
	server := &http.Server{Addr: addr, Handler: r, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("serve: %v", err)
	}
	stop()
	<-workerDone
}

func openRepo(ctx context.Context) (repository.PaymentStore, func()) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Println("DATABASE_URL unset; using in-memory payments")
		return repository.NewInMemoryPayment(), func() {}
	}
	repo, err := repository.NewPostgresPayment(ctx, dsn)
	if err != nil {
		log.Fatalf("postgres payments: %v", err)
	}
	log.Println("using postgres payments")
	return repo, repo.Close
}
