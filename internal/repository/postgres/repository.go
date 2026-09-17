package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	Pool *pgxpool.Pool
}

func NewRepository(ctx context.Context, connString string) (*Repository, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("error al parsear la configuración de PostgreSQL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("error al conectar con la base de datos PostgreSQL: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("error al hacer ping a PostgreSQL: %w", err)
	}

	return &Repository{Pool: pool}, nil
}

func (r *Repository) Close() {
	if r.Pool != nil {
		r.Pool.Close()
	}
}

// ExecTx ejecuta una función dentro de una transacción transaccional estricta de pgx
func (r *Repository) ExecTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("error al iniciar transacción: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}
