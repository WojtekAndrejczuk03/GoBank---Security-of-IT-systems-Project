package accounts

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/wojtekandrejczuk/gobank/internal/db"
)

// Account holds the essential account details returned to the caller.
type Account struct {
	AccountNumber string
	Balance       int // in grosze (1 PLN = 100 grosze)
}

// Transaction represents a single entry from the transaction history.
type Transaction struct {
	ID          int
	Type        string // "deposit", "withdrawal", "transfer"
	Amount      int    // in grosze
	Description string
	CreatedAt   time.Time
}

// GetBalance returns the account number and current balance (in grosze)
// for the given user.
func GetBalance(database *db.DB, userID int) (*Account, error) {
	var acc Account
	err := database.QueryRow(
		"SELECT account_number, balance FROM accounts WHERE user_id = ?",
		userID,
	).Scan(&acc.AccountNumber, &acc.Balance)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("nie znaleziono konta bankowego dla tego użytkownika")
	}
	if err != nil {
		return nil, fmt.Errorf("błąd odczytu salda: %w", err)
	}
	return &acc, nil
}

// Deposit adds the given amount (in grosze) to the user's account and records
// the operation in the transactions table. The whole operation is atomic.
func Deposit(database *db.DB, userID, amount int) error {
	if amount <= 0 {
		return errors.New("kwota wpłaty musi być większa niż zero")
	}

	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("błąd rozpoczęcia transakcji: %w", err)
	}
	defer tx.Rollback()

	// Fetch the account ID so we can reference it in transactions.
	var accountID int
	err = tx.QueryRow(
		"SELECT id FROM accounts WHERE user_id = ?", userID,
	).Scan(&accountID)
	if err != nil {
		return fmt.Errorf("nie znaleziono konta: %w", err)
	}

	// Credit the account.
	_, err = tx.Exec(
		"UPDATE accounts SET balance = balance + ? WHERE id = ?",
		amount, accountID,
	)
	if err != nil {
		return fmt.Errorf("błąd aktualizacji salda: %w", err)
	}

	// Record the deposit — from_account_id is NULL (money enters from outside).
	_, err = tx.Exec(
		`INSERT INTO transactions (from_account_id, to_account_id, amount, type, description)
		 VALUES (NULL, ?, ?, 'deposit', 'Wpłata środków')`,
		accountID, amount,
	)
	if err != nil {
		return fmt.Errorf("błąd zapisu transakcji: %w", err)
	}

	return tx.Commit()
}

// Transfer moves amount grosze from the sender's account to the account
// identified by toAccountNumber. Both the debit and credit happen in a single
// SQL transaction — either both succeed or neither does.
func Transfer(database *db.DB, fromUserID int, toAccountNumber string, amount int) error {
	if amount <= 0 {
		return errors.New("kwota przelewu musi być większa niż zero")
	}

	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("błąd rozpoczęcia transakcji: %w", err)
	}
	defer tx.Rollback()

	// Lock sender's row and read current balance.
	var fromAccountID, balance int
	err = tx.QueryRow(
		"SELECT id, balance FROM accounts WHERE user_id = ?", fromUserID,
	).Scan(&fromAccountID, &balance)
	if err != nil {
		return fmt.Errorf("nie znaleziono konta nadawcy: %w", err)
	}

	if balance < amount {
		return errors.New("niewystarczające środki na koncie")
	}

	// Resolve the recipient's account.
	var toAccountID int
	err = tx.QueryRow(
		"SELECT id FROM accounts WHERE account_number = ?", toAccountNumber,
	).Scan(&toAccountID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("nie znaleziono konta odbiorcy: %s", toAccountNumber)
	}
	if err != nil {
		return fmt.Errorf("błąd wyszukiwania konta odbiorcy: %w", err)
	}

	if fromAccountID == toAccountID {
		return errors.New("nie można wykonać przelewu na własne konto")
	}

	// Debit sender.
	if _, err = tx.Exec(
		"UPDATE accounts SET balance = balance - ? WHERE id = ?",
		amount, fromAccountID,
	); err != nil {
		return fmt.Errorf("błąd obciążenia konta nadawcy: %w", err)
	}

	// Credit recipient.
	if _, err = tx.Exec(
		"UPDATE accounts SET balance = balance + ? WHERE id = ?",
		amount, toAccountID,
	); err != nil {
		return fmt.Errorf("błąd uznania konta odbiorcy: %w", err)
	}

	// Record the transfer for both sides in one row.
	_, err = tx.Exec(
		`INSERT INTO transactions (from_account_id, to_account_id, amount, type, description)
		 VALUES (?, ?, ?, 'transfer', 'Przelew bankowy')`,
		fromAccountID, toAccountID, amount,
	)
	if err != nil {
		return fmt.Errorf("błąd zapisu transakcji: %w", err)
	}

	return tx.Commit()
}

// History returns the last `limit` transactions involving the user's account,
// ordered from newest to oldest.
func History(database *db.DB, userID, limit int) ([]Transaction, error) {
	var accountID int
	err := database.QueryRow(
		"SELECT id FROM accounts WHERE user_id = ?", userID,
	).Scan(&accountID)
	if err != nil {
		return nil, fmt.Errorf("nie znaleziono konta: %w", err)
	}

	rows, err := database.Query(
		`SELECT id, type, amount, COALESCE(description, ''), created_at
		 FROM transactions
		 WHERE from_account_id = ? OR to_account_id = ?
		 ORDER BY created_at DESC
		 LIMIT ?`,
		accountID, accountID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("błąd odczytu historii: %w", err)
	}
	defer rows.Close()

	var history []Transaction
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(&t.ID, &t.Type, &t.Amount, &t.Description, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("błąd odczytu wiersza: %w", err)
		}
		history = append(history, t)
	}
	return history, rows.Err()
}
