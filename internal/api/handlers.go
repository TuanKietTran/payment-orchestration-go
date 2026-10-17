package api

import (
	"encoding/json"
	"net/http"

	"github.com/TuanKietTran/payment-orchestration-go/internal/command"
	"github.com/TuanKietTran/payment-orchestration-go/internal/query"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Handlers struct {
	create *command.CreatePaymentHandler
	get    *query.GetPaymentHandler
}

func NewHandlers(c *command.CreatePaymentHandler, g *query.GetPaymentHandler) *Handlers {
	return &Handlers{create: c, get: g}
}

func (h *Handlers) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/payments", h.createPayment)
	r.Get("/payments/{id}", h.getPayment)
	return r
}

type createReq struct {
	IdempotencyKey string          `json:"idempotency_key"`
	SourceAccount  string          `json:"source_account"`
	DestAccount    string          `json:"dest_account"`
	Amount         decimal.Decimal `json:"amount"`
	Currency       string          `json:"currency"`
}

func (h *Handlers) createPayment(w http.ResponseWriter, r *http.Request) {
	var req createReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	id, err := h.create.Handle(r.Context(), command.CreatePaymentCommand{
		IdempotencyKey: req.IdempotencyKey,
		SourceAccount:  req.SourceAccount,
		DestAccount:    req.DestAccount,
		Amount:         req.Amount,
		Currency:       req.Currency,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"payment_id": id.String()})
}

func (h *Handlers) getPayment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	p, err := h.get.Handle(r.Context(), query.GetPaymentQuery{ID: id})
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}
