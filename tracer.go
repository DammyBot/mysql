// Go MySQL Driver - A MySQL-Driver for Go's database/sql package
//
// Copyright 2024 The Go-MySQL-Driver Authors. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at http://mozilla.org/MPL/2.0/.

package mysql

import (
	"context"
	"time"
)

// QueryInfo contains metadata about a query.
type QueryInfo struct {
	Query        string
	Args         []any
	Database     string
	IsPrepared   bool
	ConnectionID uint64
	StartTime    time.Time
}

// QueryResult contains the results of a query execution.
type QueryResult struct {
	Duration     time.Duration
	Error        error
	RowsAffected int64
	LastInsertID int64
}

// QueryTracer is the interface for SQL query tracing.
type QueryTracer interface {
	TraceQueryStart(ctx context.Context, info QueryInfo) context.Context
	TraceQueryEnd(ctx context.Context, res QueryResult)
}

// NoopTracer is a QueryTracer that does nothing.
type NoopTracer struct{}

var _ QueryTracer = NoopTracer{}

func (NoopTracer) TraceQueryStart(ctx context.Context, _ QueryInfo) context.Context {
	return ctx
}

func (NoopTracer) TraceQueryEnd(_ context.Context, _ QueryResult) {}

// FuncTracer is a QueryTracer that uses callbacks.
type FuncTracer struct {
	OnStart func(context.Context, QueryInfo) context.Context
	OnEnd   func(context.Context, QueryResult)
}

var _ QueryTracer = FuncTracer{}

func (f FuncTracer) TraceQueryStart(ctx context.Context, info QueryInfo) context.Context {
	if f.OnStart != nil {
		return f.OnStart(ctx, info)
	}
	return ctx
}

func (f FuncTracer) TraceQueryEnd(ctx context.Context, res QueryResult) {
	if f.OnEnd != nil {
		f.OnEnd(ctx, res)
	}
}
