// Go MySQL Driver - Query Tracer Tests
//
// Copyright 2024 The Go-MySQL-Driver Authors. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at http://mozilla.org/MPL/2.0.

//go:build tracer

package mysql

import (
	"context"
	"sync"
	"testing"
	"time"

	"database/sql/driver"
)

// TestNoopTracer verifies that NoopTracer is a working no-op implementation
func TestNoopTracer(t *testing.T) {
	tracer := NoopTracer{}

	ctx := context.Background()
	info := QueryInfo{
		Query:        "SELECT 1",
		Args:         []any{1, "test"},
		Database:     "testdb",
		IsPrepared:   false,
		ConnectionID: 123,
		StartTime:    time.Now(),
	}

	// TraceQueryStart should return the same context
	newCtx := tracer.TraceQueryStart(ctx, info)
	if newCtx != ctx {
		t.Errorf("NoopTracer should return the same context, got different context")
	}

	// TraceQueryEnd should not panic
	result := QueryResult{
		Duration:     time.Second,
		Error:        nil,
		RowsAffected: 1,
		LastInsertID: 100,
	}

	// This should not panic
	tracer.TraceQueryEnd(newCtx, result)
}

// TestFuncTracerNilCallbacks verifies that FuncTracer handles nil callbacks safely
func TestFuncTracerNilCallbacks(t *testing.T) {
	tracer := FuncTracer{}

	ctx := context.Background()
	info := QueryInfo{
		Query:        "SELECT 1",
		Args:         []any{1, "test"},
		Database:     "testdb",
		IsPrepared:   false,
		ConnectionID: 123,
		StartTime:    time.Now(),
	}

	// Should not panic with nil OnStart
	newCtx := tracer.TraceQueryStart(ctx, info)
	if newCtx != ctx {
		t.Errorf("FuncTracer with nil OnStart should return the same context")
	}

	result := QueryResult{
		Duration:     time.Second,
		Error:        nil,
		RowsAffected: 1,
		LastInsertID: 100,
	}

	// Should not panic with nil OnEnd
	tracer.TraceQueryEnd(newCtx, result)
}

// TestFuncTracerWithCallbacks verifies that FuncTracer calls the callbacks correctly
func TestFuncTracerWithCallbacks(t *testing.T) {
	var startCalled, endCalled bool
	var capturedInfo QueryInfo
	var capturedResult QueryResult

	tracer := FuncTracer{
		OnStart: func(ctx context.Context, info QueryInfo) context.Context {
			startCalled = true
			capturedInfo = info
			// Add a value to the context to test propagation
			return context.WithValue(ctx, "test-key", "test-value")
		},
		OnEnd: func(ctx context.Context, result QueryResult) {
			endCalled = true
			capturedResult = result
			// Verify context propagation
			if val := ctx.Value("test-key"); val != "test-value" {
				t.Errorf("Context not propagated correctly, got: %v", val)
			}
		},
	}

	ctx := context.Background()
	info := QueryInfo{
		Query:        "SELECT 1",
		Args:         []any{1, "test"},
		Database:     "testdb",
		IsPrepared:   false,
		ConnectionID: 123,
		StartTime:    time.Now(),
	}

	newCtx := tracer.TraceQueryStart(ctx, info)
	if !startCalled {
		t.Error("OnStart callback should be called")
	}
	// Verify that the captured info has the expected values
	if capturedInfo.Query != info.Query {
		t.Error("QueryInfo.Query should be captured correctly")
	}
	if capturedInfo.Database != info.Database {
		t.Error("QueryInfo.Database should be captured correctly")
	}
	if capturedInfo.IsPrepared != info.IsPrepared {
		t.Error("QueryInfo.IsPrepared should be captured correctly")
	}
	if newCtx == ctx {
		t.Error("Context should be modified by callback")
	}

	result := QueryResult{
		Duration:     time.Second,
		Error:        nil,
		RowsAffected: 1,
		LastInsertID: 100,
	}

	tracer.TraceQueryEnd(newCtx, result)
	if !endCalled {
		t.Error("OnEnd callback should be called")
	}
	// Verify that the captured result has the expected values
	if capturedResult.Duration != result.Duration {
		t.Error("QueryResult.Duration should be captured correctly")
	}
	if capturedResult.Error != result.Error {
		t.Error("QueryResult.Error should be captured correctly")
	}
}

// TestQueryTracerIntegration tests the tracer integration with actual query execution
func TestQueryTracerIntegration(t *testing.T) {
	if !available {
		t.Skip("MySQL server not available")
	}

	var startCalls, endCalls int
	var capturedQueries []string
	var mu sync.Mutex

	tracer := FuncTracer{
		OnStart: func(ctx context.Context, info QueryInfo) context.Context {
			mu.Lock()
			defer mu.Unlock()
			startCalls++
			capturedQueries = append(capturedQueries, info.Query)
			return ctx
		},
		OnEnd: func(ctx context.Context, result QueryResult) {
			mu.Lock()
			defer mu.Unlock()
			endCalls++
		},
	}

	// Create config with tracer
	cfg := NewConfig()
	cfg.User = user
	cfg.Passwd = pass
	cfg.Net = prot
	cfg.Addr = addr
	cfg.DBName = dbname

	err := cfg.Apply(TracerOption(tracer))
	if err != nil {
		t.Fatalf("TracerOption should work: %v", err)
	}

	// Create connection
	conn, err := NewConnector(cfg)
	if err != nil {
		t.Fatalf("NewConnector should work: %v", err)
	}

	dbConn, err := conn.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect should work: %v", err)
	}
	defer dbConn.Close()

	mc, ok := dbConn.(*mysqlConn)
	if !ok {
		t.Fatal("Connection should be *mysqlConn")
	}

	// Test QueryContext
	_, err = mc.QueryContext(context.Background(), "SELECT 1", nil)
	if err != nil {
		t.Fatalf("QueryContext should work: %v", err)
	}

	// Test ExecContext
	_, err = mc.ExecContext(context.Background(), "CREATE TABLE IF NOT EXISTS test_tracer (id INT)", nil)
	if err != nil {
		t.Fatalf("ExecContext should work: %v", err)
	}

	_, err = mc.ExecContext(context.Background(), "INSERT INTO test_tracer (id) VALUES (1)", nil)
	if err != nil {
		t.Fatalf("ExecContext INSERT should work: %v", err)
	}

	// Test prepared statement
	stmt, err := mc.PrepareContext(context.Background(), "SELECT * FROM test_tracer WHERE id = ?")
	if err != nil {
		t.Fatalf("PrepareContext should work: %v", err)
	}

	// Convert to *mysqlStmt to access QueryContext and ExecContext methods
	mysqlStmt, ok := stmt.(*mysqlStmt)
	if !ok {
		t.Fatal("Statement should be *mysqlStmt")
	}

	_, err = mysqlStmt.QueryContext(context.Background(), []driver.NamedValue{{
		Name:    "id",
		Ordinal: 1,
		Value:   int64(1),
	}})
	if err != nil {
		t.Fatalf("Prepared statement QueryContext should work: %v", err)
	}

	_, err = mysqlStmt.ExecContext(context.Background(), []driver.NamedValue{{
		Name:    "id",
		Ordinal: 1,
		Value:   int64(1),
	}})
	if err != nil {
		t.Fatalf("Prepared statement ExecContext should work: %v", err)
	}

	// Clean up
	_, err = mc.ExecContext(context.Background(), "DROP TABLE IF EXISTS test_tracer", nil)
	if err != nil {
		t.Fatalf("Cleanup should work: %v", err)
	}

	// Verify tracer was called
	mu.Lock()
	defer mu.Unlock()

	if startCalls <= 0 {
		t.Error("TraceQueryStart should be called")
	}
	if endCalls <= 0 {
		t.Error("TraceQueryEnd should be called")
	}
	if startCalls != endCalls {
		t.Errorf("Start and end calls should match, got %d start and %d end", startCalls, endCalls)
	}

	hasRegularQuery := false
	hasPreparedQuery := false
	for _, query := range capturedQueries {
		if query == "SELECT 1" {
			hasRegularQuery = true
		}
		if query == "PREPARED_STATEMENT" {
			hasPreparedQuery = true
		}
	}

	if !hasRegularQuery {
		t.Error("Should capture regular queries")
	}
	if !hasPreparedQuery {
		t.Error("Should capture prepared statements")
	}
}

// TestQueryTracerErrorHandling tests that errors are properly captured by the tracer
func TestQueryTracerErrorHandling(t *testing.T) {
	if !available {
		t.Skip("MySQL server not available")
	}

	var capturedErrors []error
	var mu sync.Mutex

	tracer := FuncTracer{
		OnEnd: func(ctx context.Context, result QueryResult) {
			mu.Lock()
			defer mu.Unlock()
			if result.Error != nil {
				capturedErrors = append(capturedErrors, result.Error)
			}
		},
	}

	// Create config with tracer
	cfg := NewConfig()
	cfg.User = user
	cfg.Passwd = pass
	cfg.Net = prot
	cfg.Addr = addr
	cfg.DBName = dbname

	err := cfg.Apply(TracerOption(tracer))
	if err != nil {
		t.Fatalf("TracerOption should work: %v", err)
	}

	// Create connection
	conn, err := NewConnector(cfg)
	if err != nil {
		t.Fatalf("NewConnector should work: %v", err)
	}

	dbConn, err := conn.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect should work: %v", err)
	}
	defer dbConn.Close()

	mc, ok := dbConn.(*mysqlConn)
	if !ok {
		t.Fatal("Connection should be *mysqlConn")
	}

	// Test with invalid query to trigger error
	_, err = mc.QueryContext(context.Background(), "INVALID SQL QUERY", nil)
	if err == nil {
		t.Fatal("Invalid query should return error")
	}

	mu.Lock()
	defer mu.Unlock()

	if len(capturedErrors) == 0 {
		t.Error("Tracer should capture errors")
	}
	if capturedErrors[0] != err {
		t.Errorf("Captured error should match original error, got %v, want %v", capturedErrors[0], err)
	}
}

// TestQueryTracerConcurrent tests that the tracer is thread-safe
func TestQueryTracerConcurrent(t *testing.T) {
	if !available {
		t.Skip("MySQL server not available")
	}

	var startCalls, endCalls int
	var mu sync.Mutex

	tracer := FuncTracer{
		OnStart: func(ctx context.Context, info QueryInfo) context.Context {
			mu.Lock()
			defer mu.Unlock()
			startCalls++
			return ctx
		},
		OnEnd: func(ctx context.Context, result QueryResult) {
			mu.Lock()
			defer mu.Unlock()
			endCalls++
		},
	}

	// Create config with tracer
	cfg := NewConfig()
	cfg.User = user
	cfg.Passwd = pass
	cfg.Net = prot
	cfg.Addr = addr
	cfg.DBName = dbname

	err := cfg.Apply(TracerOption(tracer))
	if err != nil {
		t.Fatalf("TracerOption should work: %v", err)
	}

	// Create connection
	conn, err := NewConnector(cfg)
	if err != nil {
		t.Fatalf("NewConnector should work: %v", err)
	}

	dbConn, err := conn.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect should work: %v", err)
	}
	defer dbConn.Close()

	mc, ok := dbConn.(*mysqlConn)
	if !ok {
		t.Fatal("Connection should be *mysqlConn")
	}

	// Create table for testing
	_, err = mc.ExecContext(context.Background(), "CREATE TABLE IF NOT EXISTS test_concurrent (id INT)", nil)
	if err != nil {
		t.Fatalf("Setup should work: %v", err)
	}
	defer func() {
		_, _ = mc.ExecContext(context.Background(), "DROP TABLE IF EXISTS test_concurrent", nil)
	}()

	// Run concurrent queries
	var wg sync.WaitGroup
	numQueries := 10

	for i := 0; i < numQueries; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := mc.QueryContext(context.Background(), "SELECT ?", []driver.NamedValue{{
				Ordinal: 1,
				Value:   id,
			}})
			if err != nil {
				t.Errorf("Concurrent query should work: %v", err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all tracer calls were made
	mu.Lock()
	defer mu.Unlock()

	if startCalls != numQueries {
		t.Errorf("All start calls should be made, got %d, want %d", startCalls, numQueries)
	}
	if endCalls != numQueries {
		t.Errorf("All end calls should be made, got %d, want %d", endCalls, numQueries)
	}
	if startCalls != endCalls {
		t.Errorf("Start and end calls should match, got %d start and %d end", startCalls, endCalls)
	}
}

// TestQueryTracerContextPropagation tests that context is properly propagated
func TestQueryTracerContextPropagation(t *testing.T) {
	tracer := FuncTracer{
		OnStart: func(ctx context.Context, info QueryInfo) context.Context {
			// Add a value to the context
			return context.WithValue(ctx, "test-key", "test-value")
		},
		OnEnd: func(ctx context.Context, result QueryResult) {
			// Verify the context value is present
			if val := ctx.Value("test-key"); val != "test-value" {
				t.Errorf("Context not propagated correctly, got: %v", val)
			}
		},
	}

	ctx := context.Background()
	info := QueryInfo{
		Query:        "SELECT 1",
		Args:         nil,
		Database:     "testdb",
		IsPrepared:   false,
		ConnectionID: 123,
		StartTime:    time.Now(),
	}

	newCtx := tracer.TraceQueryStart(ctx, info)

	result := QueryResult{
		Duration:     time.Second,
		Error:        nil,
		RowsAffected: 1,
		LastInsertID: 100,
	}

	tracer.TraceQueryEnd(newCtx, result)
	// If we get here without panic, the test passed
}

// TestTracerOptionNilTracer tests that TracerOption handles nil tracer gracefully
func TestTracerOptionNilTracer(t *testing.T) {
	cfg := NewConfig()

	// Should not panic with nil tracer
	err := cfg.Apply(TracerOption(nil))
	if err != nil {
		t.Errorf("TracerOption with nil tracer should not error: %v", err)
	}

	// Verify tracer is nil
	if cfg.tracer != nil {
		t.Error("Tracer should be nil")
	}
}
