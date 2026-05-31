package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wojtekandrejczuk/gobank/internal/crypto"
)

// Create reads the SQLite database file, encrypts it with AES-256-GCM using
// a key derived from the master password, and writes the result to a timestamped
// .enc file inside dataDir. Returns the path of the created backup file.
func Create(dataDir, masterPassword string) (string, error) {
	dbPath := filepath.Join(dataDir, "gobank.db")

	plaintext, err := os.ReadFile(dbPath)
	if err != nil {
		return "", fmt.Errorf("błąd odczytu bazy danych: %w", err)
	}

	key := crypto.DeriveKey(masterPassword)

	ciphertext, err := crypto.Encrypt(key, plaintext)
	if err != nil {
		return "", fmt.Errorf("błąd szyfrowania kopii zapasowej: %w", err)
	}

	// Timestamp in the filename so multiple backups can coexist.
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupPath := filepath.Join(dataDir, fmt.Sprintf("backup_%s.enc", timestamp))

	if err := os.WriteFile(backupPath, ciphertext, 0600); err != nil {
		return "", fmt.Errorf("błąd zapisu kopii zapasowej: %w", err)
	}

	return backupPath, nil
}

// Restore decrypts the backup file at backupPath using the master password
// and overwrites the current database file. The original database is saved
// with a .bak extension before overwriting so the operation can be undone
// manually if needed.
func Restore(dataDir, backupPath, masterPassword string) error {
	ciphertext, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("błąd odczytu pliku kopii zapasowej: %w", err)
	}

	key := crypto.DeriveKey(masterPassword)

	plaintext, err := crypto.Decrypt(key, ciphertext)
	if err != nil {
		// crypto.Decrypt already returns a Polish message for wrong key / tampered data.
		return err
	}

	dbPath := filepath.Join(dataDir, "gobank.db")

	// Keep the current database as a safety copy before overwriting.
	if _, statErr := os.Stat(dbPath); statErr == nil {
		bakPath := dbPath + ".bak"
		if err := os.Rename(dbPath, bakPath); err != nil {
			return fmt.Errorf("błąd tworzenia kopii bezpieczeństwa bazy danych: %w", err)
		}
	}

	if err := os.WriteFile(dbPath, plaintext, 0600); err != nil {
		return fmt.Errorf("błąd przywracania bazy danych: %w", err)
	}

	return nil
}
