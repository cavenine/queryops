package testdb

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cavenine/queryops/migrations"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	defaultImage    = "postgres:18.1-bookworm"
	defaultUser     = "test"
	defaultPassword = "test"
	defaultDatabase = "test"

	reuseEnvVar        = "QUERYOPS_TESTDB_REUSE"
	reuseContainerName = "queryops-testdb-postgres" // used by WithReuseByName
	snapshotName       = "queryops_migrated_template"
)

var snapshotMu sync.Mutex
var snapshotReady bool

func reuseEnabled() bool {
	v := strings.TrimSpace(os.Getenv(reuseEnvVar))
	if v == "" {
		return false
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func newTestDatabaseName() string {
	// Safe identifier: letters/digits/underscore only.
	u := strings.ReplaceAll(uuid.NewString(), "-", "")
	return "test_" + u
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

type TestDB struct {
	Container *postgres.PostgresContainer
	Pool      *pgxpool.Pool

	DSN      string
	Host     string
	Port     string
	Database string
	User     string
	Password string
}

func SetupTestDB(t *testing.T) *TestDB {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	reuse := reuseEnabled()

	containerOpts := []testcontainers.ContainerCustomizer{
		postgres.WithDatabase(defaultDatabase),
		postgres.WithUsername(defaultUser),
		postgres.WithPassword(defaultPassword),
		postgres.BasicWaitStrategies(),
		testcontainers.WithAdditionalWaitStrategyAndDeadline(
			90*time.Second,
			wait.ForListeningPort("5432/tcp"),
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	}
	if reuse {
		// Experimental: keep container around across `go test` runs.
		containerOpts = append(containerOpts,
			testcontainers.WithReuseByName(reuseContainerName),
			postgres.WithSQLDriver("pgx/v5"),
		)
	}

	container, err := postgres.Run(ctx, defaultImage, containerOpts...)
	if err != nil {
		// Common when Docker isn't available (some CI/dev environments).
		t.Skipf("starting postgres testcontainer: %v", err)
		return nil
	}

	if !reuse {
		t.Cleanup(func() {
			termCtx, termCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer termCancel()
			_ = container.Terminate(termCtx)
		})
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("getting container host: %v", err)
	}
	mappedPort, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("getting mapped port: %v", err)
	}

	port := mappedPort.Port()
	baseDSN := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", defaultUser, defaultPassword, host, port, defaultDatabase)
	dsn := baseDSN
	dbName := defaultDatabase

	if reuse {
		ensureMigratedSnapshot(t, ctx, container, baseDSN)

		dbName = newTestDatabaseName()

		adminCfg, err := pgxpool.ParseConfig(baseDSN)
		if err != nil {
			t.Fatalf("parsing admin pool config: %v", err)
		}
		adminCfg.ConnConfig.Database = "postgres"
		adminCfg.MinConns = 0
		adminCfg.MaxConns = 1

		adminPool, err := pgxpool.NewWithConfig(ctx, adminCfg)
		if err != nil {
			t.Fatalf("creating admin pool: %v", err)
		}
		defer adminPool.Close()

		createStmt := fmt.Sprintf(
			"CREATE DATABASE %s WITH TEMPLATE %s OWNER %s",
			quoteIdent(dbName),
			quoteIdent(snapshotName),
			quoteIdent(defaultUser),
		)
		if _, err := adminPool.Exec(ctx, createStmt); err != nil {
			t.Fatalf("creating test db %q from template: %v", dbName, err)
		}

		dsn = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", defaultUser, defaultPassword, host, port, dbName)

		// Drop the per-test database but keep the reused container.
		// Register before pool.Close so it runs after.
		t.Cleanup(func() {
			dropCtx, dropCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer dropCancel()

			cfg, err := pgxpool.ParseConfig(baseDSN)
			if err != nil {
				return
			}
			cfg.ConnConfig.Database = "postgres"
			cfg.MinConns = 0
			cfg.MaxConns = 1

			p, err := pgxpool.NewWithConfig(dropCtx, cfg)
			if err != nil {
				return
			}
			defer p.Close()

			dropStmt := "DROP DATABASE IF EXISTS " + quoteIdent(dbName) + " WITH (FORCE)"
			if _, err := p.Exec(dropCtx, dropStmt); err != nil {
				_, _ = p.Exec(dropCtx, "DROP DATABASE IF EXISTS "+quoteIdent(dbName))
			}
		})
	}

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parsing pgx pool config: %v", err)
	}
	poolCfg.MinConns = 0
	poolCfg.MaxConns = 4

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatalf("creating pgx pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("pinging db: %v", err)
	}

	t.Cleanup(pool.Close)

	if !reuse {
		if err := migrations.Up(dsn); err != nil {
			t.Fatalf("applying migrations: %v", err)
		}

		migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
		if err != nil {
			t.Fatalf("creating river migrator: %v", err)
		}
		if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
			t.Fatalf("running river migrations: %v", err)
		}
	}

	return &TestDB{
		Container: container,
		Pool:      pool,
		DSN:       dsn,
		Host:      host,
		Port:      port,
		Database:  dbName,
		User:      defaultUser,
		Password:  defaultPassword,
	}
}

func ensureMigratedSnapshot(t *testing.T, ctx context.Context, container *postgres.PostgresContainer, dsn string) {
	t.Helper()

	snapshotMu.Lock()
	defer snapshotMu.Unlock()
	if snapshotReady {
		return
	}

	// Snapshot/restore uses a template database. Detect it by checking pg_database
	// from the system database.
	adminCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parsing admin pool config: %v", err)
	}
	adminCfg.ConnConfig.Database = "postgres"
	adminCfg.MinConns = 0
	adminCfg.MaxConns = 1

	adminPool, err := pgxpool.NewWithConfig(ctx, adminCfg)
	if err != nil {
		t.Fatalf("creating admin pool: %v", err)
	}
	defer adminPool.Close()

	if err := adminPool.Ping(ctx); err != nil {
		t.Fatalf("pinging admin db: %v", err)
	}

	var baseDBExists bool
	if err := adminPool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, defaultDatabase).Scan(&baseDBExists); err != nil {
		t.Fatalf("checking base db exists: %v", err)
	}
	if !baseDBExists {
		if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+defaultDatabase); err != nil {
			t.Fatalf("creating base db %q: %v", defaultDatabase, err)
		}
	}

	var snapshotExists bool
	if err := adminPool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, snapshotName).Scan(&snapshotExists); err != nil {
		t.Fatalf("checking snapshot db exists: %v", err)
	}
	if snapshotExists {
		snapshotReady = true
		return
	}

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parsing pgx pool config for snapshot init: %v", err)
	}
	poolCfg.MinConns = 0
	poolCfg.MaxConns = 4

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatalf("creating pgx pool for snapshot init: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("pinging db for snapshot init: %v", err)
	}

	if err := migrations.Up(dsn); err != nil {
		t.Fatalf("applying migrations for snapshot init: %v", err)
	}

	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		t.Fatalf("creating river migrator for snapshot init: %v", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		t.Fatalf("running river migrations for snapshot init: %v", err)
	}

	// Important: Snapshot uses the source DB as a TEMPLATE; it cannot have active connections.
	pool.Close()

	snapOpts := []postgres.SnapshotOption{postgres.WithSnapshotName(snapshotName)}

	for attempt := 1; attempt <= 5; attempt++ {
		err = container.Snapshot(ctx, snapOpts...)
		if err == nil {
			snapshotReady = true
			return
		}
		// Transient when connections are still draining.
		if strings.Contains(err.Error(), "being accessed by other users") {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		break
	}
	if err != nil {
		t.Fatalf("creating snapshot: %v", err)
	}
	snapshotReady = true
}
