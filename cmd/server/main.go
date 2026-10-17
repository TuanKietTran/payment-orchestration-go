package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/example/payment-orchestration/internal/api"
	"github.com/example/payment-orchestration/internal/command"
	"github.com/example/payment-orchestration/internal/domain"
	"github.com/example/payment-orchestration/internal/query"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

type memRepo struct {
	store map[uuid.UUID]domain.Payment
}

func (m *memRepo) Save(ctx context.Context, p domain.Payment) error {
	if m.store == nil {
		m.store = make(map[uuid.UUID]domain.Payment)
	}
	m.store[p.ID] = p
	return nil
}
func (m *memRepo) GetByID(ctx context.Context, id domain.PaymentID) (*domain.Payment, error) {
	p, ok := m.store[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return &p, nil
}
func (m *memRepo) UpdateStatus(ctx context.Context, id domain.PaymentID, status domain.PaymentStatus) error {
	return nil
}

func main() {
	repo := &memRepo{}
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
