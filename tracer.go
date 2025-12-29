// Go MySQL Driver - Query Tracer Implementation
//
// Copyright 2024 The Go-MySQL-Driver Authors. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at http://mozilla.org/MPL/2.0.

package mysql

import (
	"context"
	"time"
)

// QueryTracer interface for SQL query logging and monitoring
type QueryTracer interface {
	// TraceQueryStart is called when a query is about to be executed
	TraceQueryStart(context.Context, QueryInfo) context.Context

	// TraceQueryEnd is called when a query has completed execution
	TraceQueryEnd(context.Context, QueryResult)
}

// QueryInfo contains information about a query being executed
type QueryInfo struct {
	Query        string    // The SQL query being executed
	Args         []any     // Query arguments
	Database     string    // Database name
	IsPrepared   bool      // Whether this is a prepared statement execution
	ConnectionID uint64    // Connection ID
	StartTime    time.Time // When the query started
}

// QueryResult contains the result of query execution
type QueryResult struct {
	Duration     time.Duration // Query execution duration
	Error        error         // Error from query execution (nil if successful)
	RowsAffected int64         // Number of rows affected
	LastInsertID int64         // Last insert ID (for INSERT queries)
}

// NoopTracer is an empty implementation of QueryTracer that does nothing
type NoopTracer struct{}

// TraceQueryStart implements QueryTracer interface (no-op)
func (NoopTracer) TraceQueryStart(ctx context.Context, _ QueryInfo) context.Context {
	return ctx
}

// TraceQueryEnd implements QueryTracer interface (no-op)
func (NoopTracer) TraceQueryEnd(_ context.Context, _ QueryResult) {}

// FuncTracer is a callback-based implementation of QueryTracer
type FuncTracer struct {
	OnStart func(context.Context, QueryInfo) context.Context
	OnEnd   func(context.Context, QueryResult)
}

// TraceQueryStart implements QueryTracer interface with nil-safe callback
func (ft FuncTracer) TraceQueryStart(ctx context.Context, info QueryInfo) context.Context {
	if ft.OnStart != nil {
		return ft.OnStart(ctx, info)
	}
	return ctx
}

// TraceQueryEnd implements QueryTracer interface with nil-safe callback
func (ft FuncTracer) TraceQueryEnd(ctx context.Context, result QueryResult) {
	if ft.OnEnd != nil {
		ft.OnEnd(ctx, result)
	}
}

// TracerOption creates a functional option to configure a QueryTracer
func TracerOption(tracer QueryTracer) Option {
	return func(cfg *Config) error {
		cfg.tracer = tracer
		return nil
	}
}
