package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/example/payment-orchestration/internal/api"
	"github.com/example/payment-orchestration/internal/command"
	"github.com/example/payment-orchestration/internal/query"
	"github.com/example/payment-orchestration/internal/repository"
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

	log.Println("Payment API listening on :8081")
	log.Fatal(http.ListenAndServe(":8081", r))
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
