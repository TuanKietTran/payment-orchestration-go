package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/TuanKietTran/payment-orchestration-go/internal/command"
	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/TuanKietTran/payment-orchestration-go/internal/query"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Handlers struct {
	create  *command.CreatePaymentHandler
	get     *query.GetPaymentHandler
	process *command.ProcessPaymentHandler
	refund  *command.RefundPaymentHandler
}

func NewHandlers(c *command.CreatePaymentHandler, g *query.GetPaymentHandler, process ...*command.ProcessPaymentHandler) *Handlers {
	h := &Handlers{create: c, get: g}
	if len(process) > 0 {
		h.process = process[0]
	}
	return h
}

func (h *Handlers) WithRefund(refund *command.RefundPaymentHandler) *Handlers {
	h.refund = refund
	return h
}

func (h *Handlers) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/payments", h.createPayment)
	r.Get("/payments/{id}", h.getPayment)
	if h.process != nil {
		r.Post("/payments/{id}/process", h.processPayment)
	}
	if h.refund != nil {
		r.Post("/payments/{id}/refunds", h.refundPayment)
	}
	return r
}

func (h *Handlers) refundPayment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var req struct {
		IdempotencyKey string          `json:"idempotency_key"`
		Amount         decimal.Decimal `json:"amount"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if header := r.Header.Get("Idempotency-Key"); header != "" {
		if req.IdempotencyKey != "" && req.IdempotencyKey != header {
			http.Error(w, "idempotency key disagreement", http.StatusBadRequest)
			return
		}
		req.IdempotencyKey = header
	}
	refund, err := h.refund.Handle(r.Context(), id, req.IdempotencyKey, req.Amount)
	if err != nil {
		status := http.StatusServiceUnavailable
		if errors.Is(err, domain.ErrRefundConflict) {
			status = http.StatusConflict
		} else if errors.Is(err, domain.ErrInvalidRefund) {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(refund)
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
		if errors.Is(err, command.ErrIdempotencyConflict) {
			http.Error(w, err.Error(), http.StatusConflict)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"payment_id": id.String()})
}

func (h *Handlers) processPayment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	p, err := h.process.Handle(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p)
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
