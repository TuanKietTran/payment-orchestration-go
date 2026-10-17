package main

import (
	"log"
	"net/http"

	"github.com/example/payment-orchestration/internal/api"
	"github.com/example/payment-orchestration/internal/command"
	"github.com/example/payment-orchestration/internal/query"
	"github.com/example/payment-orchestration/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	repo := repository.NewInMemoryPayment()
	createH := command.NewCreatePaymentHandler(repo)
	getH := query.NewGetPaymentHandler(repo)
	handlers := api.NewHandlers(createH, getH)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Mount("/v1", handlers.Routes())

	log.Println("Payment API (in-memory) listening on :8081")
	log.Fatal(http.ListenAndServe(":8081", r))
}
