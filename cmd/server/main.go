package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/TuanKietTran/payment-orchestration-go/internal/api"
	"github.com/TuanKietTran/payment-orchestration-go/internal/command"
	"github.com/TuanKietTran/payment-orchestration-go/internal/query"
	"github.com/TuanKietTran/payment-orchestration-go/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	repo, cleanup := openRepo(context.Background())
	defer cleanup()

	createH := command.NewCreatePaymentHandler(repo)
	getH := query.NewGetPaymentHandler(repo)
	handlers := api.NewHandlers(createH, getH)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Mount("/v1", handlers.Routes())

	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8102"
	}
	log.Printf("Payment API listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

func openRepo(ctx context.Context) (repository.PaymentRepository, func()) {
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
