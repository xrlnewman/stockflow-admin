package database

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	return db, nil
}

func Ping(ctx context.Context, db *sql.DB) error { return db.PingContext(ctx) }
