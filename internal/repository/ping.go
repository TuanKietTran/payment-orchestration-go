package repository

import "context"

// Ping reports readiness; the in-memory store is always available.
func (r *InMemoryPayment) Ping(context.Context) error { return nil }

// Ping verifies the Postgres pool can reach the database.
func (r *PostgresPayment) Ping(ctx context.Context) error { return r.pool.Ping(ctx) }
