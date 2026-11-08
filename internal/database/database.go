package database

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

func Migrate(ctx context.Context, pool *pgxpool.Pool, direction string) error {
	goose.SetBaseFS(migrations)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	switch direction {
	case "up":
		return goose.UpContext(ctx, db, "migrations")
	case "down":
		return goose.DownContext(ctx, db, "migrations")
	case "status":
		return goose.StatusContext(ctx, db, "migrations")
	default:
		return fmt.Errorf("unknown migrate direction %q", direction)
	}
}

// InTx runs fn inside a transaction and returns the queries bound to it.
// The transaction is committed only when fn returns nil.
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(q *sqlc.Queries, tx pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(sqlc.New(tx), tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
