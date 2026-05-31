package auth

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/wojtekandrejczuk/gobank/internal/db"
)

const bcryptCost = 12

// User holds the data returned after a successful login.
type User struct {
	ID       int
	Username string
}

// Register creates a new user account with a bcrypt-hashed password.
// It also opens a bank account for the user in the same transaction so
// the two records are always created together or not at all.
func Register(database *db.DB, username, password string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("nazwa użytkownika nie może być pusta")
	}
	if len(password) < 8 {
		return errors.New("hasło musi mieć co najmniej 8 znaków")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return fmt.Errorf("błąd hashowania hasła: %w", err)
	}

	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("błąd rozpoczęcia transakcji: %w", err)
	}
	defer tx.Rollback() //nolint — rollback is a no-op after Commit

	// Insert the user record.
	result, err := tx.Exec(
		"INSERT INTO users (username, password_hash) VALUES (?, ?)",
		username, string(hash),
	)
	if err != nil {
		return fmt.Errorf("użytkownik '%s' już istnieje", username)
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("błąd odczytu ID użytkownika: %w", err)
	}

	// Generate a unique 26-digit account number prefixed with "PL".
	accountNumber, err := generateAccountNumber()
	if err != nil {
		return fmt.Errorf("błąd generowania numeru konta: %w", err)
	}

	_, err = tx.Exec(
		"INSERT INTO accounts (user_id, account_number, balance) VALUES (?, ?, 0)",
		userID, accountNumber,
	)
	if err != nil {
		return fmt.Errorf("błąd tworzenia konta bankowego: %w", err)
	}

	return tx.Commit()
}

// Login verifies the username and password. Returns the User on success.
func Login(database *db.DB, username, password string) (*User, error) {
	username = strings.TrimSpace(username)

	var user User
	var hash string
	err := database.QueryRow(
		"SELECT id, username, password_hash FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &hash)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("nieprawidłowa nazwa użytkownika lub hasło")
	}
	if err != nil {
		return nil, fmt.Errorf("błąd bazy danych: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		// Return a generic message — don't reveal whether the username exists.
		return nil, errors.New("nieprawidłowa nazwa użytkownika lub hasło")
	}

	return &user, nil
}

// generateAccountNumber returns a "PL" + 26 random decimal digits string,
// loosely mimicking a Polish IBAN format for display purposes.
func generateAccountNumber() (string, error) {
	const digits = 26
	var sb strings.Builder
	sb.WriteString("PL")
	for i := 0; i < digits; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		sb.WriteByte(byte('0' + n.Int64()))
	}
	return sb.String(), nil
}
