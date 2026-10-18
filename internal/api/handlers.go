package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/TuanKietTran/payment-orchestration-go/internal/command"
	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/TuanKietTran/payment-orchestration-go/internal/query"
	"github.com/TuanKietTran/payment-orchestration-go/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Handlers struct {
	create  *command.CreatePaymentHandler
	get     *query.GetPaymentHandler
	process *command.ProcessPaymentHandler
	refund  *command.RefundPaymentHandler
	lister  repository.PaymentLister
	recon   *query.ReconcileHandler
}

const (
	defaultPageSize = 50
	maxPageSize     = 200
)

// WithListing enables paginated listing and ledger reconciliation routes.
func (h *Handlers) WithListing(l repository.PaymentLister, rec *query.ReconcileHandler) *Handlers {
	h.lister, h.recon = l, rec
	return h
}

func pageParams(r *http.Request) (uuid.UUID, int, bool) {
	after := uuid.Nil
	if raw := r.URL.Query().Get("after"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return after, 0, false
		}
		after = id
	}
	limit := defaultPageSize
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > maxPageSize {
			return after, 0, false
		}
		limit = n
	}
	return after, limit, true
}

func (h *Handlers) listPayments(w http.ResponseWriter, r *http.Request) {
	after, limit, ok := pageParams(r)
	if !ok {
		http.Error(w, "invalid after or limit", http.StatusBadRequest)
		return
	}
	page, err := h.lister.List(r.Context(), after, limit+1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	out := struct {
		Payments  []domain.Payment `json:"payments"`
		NextAfter string           `json:"next_after,omitempty"`
	}{Payments: page}
	if len(page) > limit {
		out.Payments = page[:limit]
		out.NextAfter = page[limit-1].ID.String()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (h *Handlers) reconcile(w http.ResponseWriter, r *http.Request) {
	after, limit, ok := pageParams(r)
	if !ok {
		http.Error(w, "invalid after or limit", http.StatusBadRequest)
		return
	}
	report, err := h.recon.Handle(r.Context(), after, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
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
		r.Post("/payments/{id}/capture", h.processPayment)
	}
	if h.lister != nil {
		r.Get("/payments", h.listPayments)
	}
	if h.recon != nil {
		r.Get("/reconciliation", h.reconcile)
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
