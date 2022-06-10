package main

import (
	"context"
	"database/sql"
	"time"
)

func migrate(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS cobbleday (
		id INTEGER PRIMARY KEY,
		hashed TEXT,
		message VARCHAR(500),
		created_at TIMESTAMP
	)`)
	if err != nil {
		return err
	}

	return nil
}
