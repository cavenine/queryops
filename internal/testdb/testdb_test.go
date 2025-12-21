package testdb

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSetupTestDB_Select1(t *testing.T) {
	tdb := SetupTestDB(t)
	if tdb == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var got int
	if err := tdb.Pool.QueryRow(ctx, "SELECT 1").Scan(&got); err != nil {
		t.Fatalf("query SELECT 1: %v", err)
	}
	if got != 1 {
		t.Fatalf("SELECT 1 returned %d", got)
	}
}

func TestSetupTestDB_ParallelReuse(t *testing.T) {
	if !reuseEnabled() {
		t.Skip("set QUERYOPS_TESTDB_REUSE=1 to run")
	}

	for i := range 3 {
		i := i
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			t.Parallel()

			tdb := SetupTestDB(t)
			if tdb == nil {
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			var got int
			if err := tdb.Pool.QueryRow(ctx, "SELECT 1").Scan(&got); err != nil {
				t.Fatalf("query SELECT 1: %v", err)
			}
			if got != 1 {
				t.Fatalf("SELECT 1 returned %d", got)
			}
		})
	}
}

func TestSetupTestDB_IsolationBetweenDatabases(t *testing.T) {
	if !reuseEnabled() {
		t.Skip("set QUERYOPS_TESTDB_REUSE=1 to run")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dbA := SetupTestDB(t)
	if dbA == nil {
		return
	}

	if _, err := dbA.Pool.Exec(ctx, `CREATE TABLE temp_isolation_check (id INT PRIMARY KEY)`); err != nil {
		t.Fatalf("create table in dbA: %v", err)
	}

	var regA *string
	if err := dbA.Pool.QueryRow(ctx, `SELECT to_regclass('public.temp_isolation_check')`).Scan(&regA); err != nil {
		t.Fatalf("check table exists in dbA: %v", err)
	}
	if regA == nil {
		t.Fatalf("expected table to exist in dbA")
	}

	dbB := SetupTestDB(t)
	if dbB == nil {
		return
	}

	var regB *string
	if err := dbB.Pool.QueryRow(ctx, `SELECT to_regclass('public.temp_isolation_check')`).Scan(&regB); err != nil {
		t.Fatalf("check table exists in dbB: %v", err)
	}
	if regB != nil {
		t.Fatalf("expected table to NOT exist in dbB, got %q", *regB)
	}
}

func TestSetupTestDB_DropsDatabaseOnCleanup(t *testing.T) {
	if !reuseEnabled() {
		t.Skip("set QUERYOPS_TESTDB_REUSE=1 to run")
	}

	var host, port, user, password, dbName string

	t.Run("create_db", func(t *testing.T) {
		tdb := SetupTestDB(t)
		if tdb == nil {
			return
		}
		host, port, user, password, dbName = tdb.Host, tdb.Port, tdb.User, tdb.Password, tdb.Database
		if dbName == defaultDatabase {
			t.Fatalf("expected per-test database name, got %q", dbName)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	adminDSN := fmt.Sprintf("postgres://%s:%s@%s:%s/postgres?sslmode=disable", user, password, host, port)
	cfg, err := pgxpool.ParseConfig(adminDSN)
	if err != nil {
		t.Fatalf("parse admin dsn: %v", err)
	}
	cfg.MinConns = 0
	cfg.MaxConns = 1

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect admin pool: %v", err)
	}
	defer pool.Close()

	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, dbName).Scan(&exists); err != nil {
		t.Fatalf("check db exists: %v", err)
	}
	if exists {
		t.Fatalf("expected database %q to be dropped", dbName)
	}
}
