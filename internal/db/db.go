package db

import (
	"context"
	"database/sql"
	"errors"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func Open(cfg Config) (*DB, error) {
	if cfg.DSN == "" {
		return nil, errors.New("missing_sqlite_dsn")
	}
	conn, err := sql.Open("sqlite", cfg.DSN)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(cfg.MaxOpenConns)
	conn.SetMaxIdleConns(cfg.MaxIdleConns)
	conn.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	return &DB{DB: conn}, nil
}

func (db *DB) Ping(ctx context.Context) error {
	if db == nil || db.DB == nil {
		return errors.New("db_not_initialized")
	}
	return db.DB.PingContext(ctx)
}
