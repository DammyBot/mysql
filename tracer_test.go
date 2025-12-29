package mysql

import (
	"context"
	"database/sql"
	"testing"
)

func TestQueryTracer(t *testing.T) {
	if !available {
		t.Skip("MySQL server not available")
	}

	type tracerKey struct{}

	var startInfo QueryInfo
	var endResult QueryResult
	var startCalled, endCalled bool

	tracer := FuncTracer{
		OnStart: func(ctx context.Context, info QueryInfo) context.Context {
			startCalled = true
			startInfo = info
			return context.WithValue(ctx, tracerKey{}, "tracer-val")
		},
		OnEnd: func(ctx context.Context, res QueryResult) {
			endCalled = true
			endResult = res
			val := ctx.Value(tracerKey{})
			if val != "tracer-val" {
				t.Errorf("expected context value 'tracer-val', got %v", val)
			}
		},
	}

	cfg, err := ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Apply(TracerOption(tracer))

	connector := newConnector(cfg)
	db := sql.OpenDB(connector)
	defer db.Close()

	ctx := context.Background()
	_, err = db.ExecContext(ctx, "CREATE TEMPORARY TABLE tracer_test (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(255))")
	if err != nil {
		t.Fatal(err)
	}

	if !startCalled || !endCalled {
		t.Fatal("expected tracer to be called")
	}
	if startInfo.Query != "CREATE TEMPORARY TABLE tracer_test (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(255))" {
		t.Errorf("unexpected query: %s", startInfo.Query)
	}
	if startInfo.IsPrepared {
		t.Error("expected IsPrepared to be false")
	}
	if startInfo.ConnectionID == 0 {
		t.Error("expected non-zero ConnectionID")
	}

	// Test Insert
	startCalled, endCalled = false, false
	_, err = db.ExecContext(ctx, "INSERT INTO tracer_test (name) VALUES (?)", "test")
	if err != nil {
		t.Fatal(err)
	}
	if !startCalled || !endCalled {
		t.Fatal("expected tracer to be called for insert")
	}
	if len(startInfo.Args) != 1 || startInfo.Args[0] != "test" {
		t.Errorf("unexpected args: %v", startInfo.Args)
	}
	if endResult.RowsAffected != 1 {
		t.Errorf("expected 1 row affected, got %d", endResult.RowsAffected)
	}
	if endResult.LastInsertID == 0 {
		t.Errorf("expected non-zero last insert ID")
	}

	// Test Query
	startCalled, endCalled = false, false
	rows, err := db.QueryContext(ctx, "SELECT name FROM tracer_test WHERE id = ?", 1)
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if !startCalled || !endCalled {
		t.Fatal("expected tracer to be called for query")
	}
	if len(startInfo.Args) != 1 || startInfo.Args[0] != 1 {
		t.Errorf("unexpected args: %v", startInfo.Args)
	}

	// Test Prepared Stmt
	startCalled, endCalled = false, false
	stmt, err := db.PrepareContext(ctx, "SELECT name FROM tracer_test WHERE id = ?")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	rows, err = stmt.QueryContext(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if !startCalled || !endCalled {
		t.Fatal("expected tracer to be called for prepared query")
	}
	if !startInfo.IsPrepared {
		t.Error("expected IsPrepared to be true")
	}
	if startInfo.Query != "SELECT name FROM tracer_test WHERE id = ?" {
		t.Errorf("unexpected query: %s", startInfo.Query)
	}
}
