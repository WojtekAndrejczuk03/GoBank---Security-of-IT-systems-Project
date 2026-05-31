package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // SQLite driver registration
)

// DB wraps the standard sql.DB with our app-specific helpers.
type DB struct {
	*sql.DB
}

// Open opens (or creates) the SQLite database at the given path,
// creates the data directory if needed, and runs all migrations.
func Open(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("nie można utworzyć katalogu danych: %w", err)
	}

	dbPath := filepath.Join(dataDir, "gobank.db")
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("nie można otworzyć bazy danych: %w", err)
	}

	// Enforce foreign key constraints — SQLite disables them by default.
	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("nie można włączyć kluczy obcych: %w", err)
	}

	db := &DB{sqlDB}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migracja bazy danych nie powiodła się: %w", err)
	}

	return db, nil
}

// migrate creates all required tables if they do not already exist.
// Adding new tables here keeps the schema evolution in one place.
func (db *DB) migrate() error {
	schema := `
	-- Stores registered users with their hashed passwords.
	CREATE TABLE IF NOT EXISTS users (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		username      TEXT    NOT NULL UNIQUE,
		password_hash TEXT    NOT NULL,
		created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Stores bank accounts; balance is kept in grosze (1 PLN = 100 grosze)
	-- to avoid floating-point rounding errors.
	CREATE TABLE IF NOT EXISTS accounts (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id        INTEGER NOT NULL REFERENCES users(id),
		account_number TEXT    NOT NULL UNIQUE,
		balance        INTEGER NOT NULL DEFAULT 0,
		created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Full audit log of every financial operation.
	-- from_account_id is NULL for deposits (money comes from outside).
	-- to_account_id   is NULL for withdrawals (money leaves the system).
	CREATE TABLE IF NOT EXISTS transactions (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		from_account_id INTEGER REFERENCES accounts(id),
		to_account_id   INTEGER REFERENCES accounts(id),
		amount          INTEGER NOT NULL,
		type            TEXT    NOT NULL CHECK(type IN ('deposit','withdrawal','transfer')),
		description     TEXT,
		created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := db.Exec(schema)
	return err
}
