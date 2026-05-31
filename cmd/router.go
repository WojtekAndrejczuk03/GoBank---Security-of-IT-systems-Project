package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/term"

	"github.com/wojtekandrejczuk/gobank/internal/accounts"
	"github.com/wojtekandrejczuk/gobank/internal/auth"
	"github.com/wojtekandrejczuk/gobank/internal/backup"
	"github.com/wojtekandrejczuk/gobank/internal/db"
)

// session stores the currently logged-in user between CLI invocations.
// It is persisted as JSON in data/session.json (permissions 0600).
type session struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
}

// Run parses args and dispatches to the appropriate command handler.
func Run(database *db.DB, dataDir string, args []string) error {
	if len(args) == 0 {
		fmt.Println(RenderHelp())
		return nil
	}

	sessionPath := filepath.Join(dataDir, "session.json")

	fmt.Println()
	switch args[0] {
	case "register":
		return cmdRegister(database)
	case "login":
		return cmdLogin(database, sessionPath)
	case "logout":
		return cmdLogout(sessionPath)
	case "balance":
		return withSession(sessionPath, func(s *session) error {
			return cmdBalance(database, s.UserID)
		})
	case "deposit":
		return withSession(sessionPath, func(s *session) error {
			return cmdDeposit(database, s.UserID)
		})
	case "transfer":
		return withSession(sessionPath, func(s *session) error {
			return cmdTransfer(database, s.UserID)
		})
	case "history":
		return withSession(sessionPath, func(s *session) error {
			return cmdHistory(database, s.UserID)
		})
	case "backup":
		return withSession(sessionPath, func(s *session) error {
			return cmdBackup(dataDir)
		})
	case "restore":
		if len(args) < 2 {
			return errors.New("użycie: gobank restore <plik_kopii>")
		}
		return cmdRestore(dataDir, args[1])
	default:
		PrintError(fmt.Sprintf("Nieznana komenda: %s", args[0]))
		fmt.Println()
		fmt.Println(RenderHelp())
		return nil
	}
}

// --- Command implementations ---

func cmdRegister(database *db.DB) error {
	username, err := prompt("  Nazwa użytkownika: ")
	if err != nil {
		return err
	}
	password, err := promptPassword("  Hasło (min. 8 znaków): ")
	if err != nil {
		return err
	}
	confirm, err := promptPassword("  Potwierdź hasło: ")
	if err != nil {
		return err
	}
	if password != confirm {
		return errors.New("hasła nie są zgodne")
	}

	if err := auth.Register(database, username, password); err != nil {
		return err
	}
	PrintSuccess(fmt.Sprintf("Konto '%s' zostało utworzone pomyślnie.", username))
	return nil
}

func cmdLogin(database *db.DB, sessionPath string) error {
	username, err := prompt("  Nazwa użytkownika: ")
	if err != nil {
		return err
	}
	password, err := promptPassword("  Hasło: ")
	if err != nil {
		return err
	}

	user, err := auth.Login(database, username, password)
	if err != nil {
		return err
	}

	s := session{UserID: user.ID, Username: user.Username}
	data, _ := json.Marshal(s)
	if err := os.WriteFile(sessionPath, data, 0600); err != nil {
		return fmt.Errorf("błąd zapisu sesji: %w", err)
	}

	PrintSuccess(fmt.Sprintf("Zalogowano jako '%s'.", user.Username))
	return nil
}

func cmdLogout(sessionPath string) error {
	if err := os.Remove(sessionPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("błąd wylogowania: %w", err)
	}
	PrintSuccess("Wylogowano pomyślnie.")
	return nil
}

func cmdBalance(database *db.DB, userID int) error {
	acc, err := accounts.GetBalance(database, userID)
	if err != nil {
		return err
	}
	fmt.Println()
	fmt.Println(RenderBalance(acc))
	return nil
}

func cmdDeposit(database *db.DB, userID int) error {
	raw, err := prompt("  Kwota wpłaty (PLN): ")
	if err != nil {
		return err
	}
	amount, err := parsePLN(raw)
	if err != nil {
		return err
	}

	if err := accounts.Deposit(database, userID, amount); err != nil {
		return err
	}
	PrintSuccess(fmt.Sprintf("Wpłacono %s.", formatPLN(amount)))
	return nil
}

func cmdTransfer(database *db.DB, userID int) error {
	target, err := prompt("  Numer konta odbiorcy: ")
	if err != nil {
		return err
	}
	raw, err := prompt("  Kwota przelewu (PLN): ")
	if err != nil {
		return err
	}
	amount, err := parsePLN(raw)
	if err != nil {
		return err
	}

	targetTrimmed := strings.TrimSpace(target)
	if err := accounts.Transfer(database, userID, targetTrimmed, amount); err != nil {
		return err
	}
	PrintSuccess(fmt.Sprintf("Przelew %s na konto %s wykonany pomyślnie.", formatPLN(amount), targetTrimmed))
	return nil
}

func cmdHistory(database *db.DB, userID int) error {
	txns, err := accounts.History(database, userID, 20)
	if err != nil {
		return err
	}
	fmt.Println()
	fmt.Println(RenderHistory(txns))
	return nil
}

func cmdBackup(dataDir string) error {
	password, err := promptPassword("  Hasło do zaszyfrowania kopii: ")
	if err != nil {
		return err
	}

	path, err := backup.Create(dataDir, password)
	if err != nil {
		return err
	}
	PrintSuccess(fmt.Sprintf("Kopia zapasowa zapisana: %s", path))
	return nil
}

func cmdRestore(dataDir, backupFile string) error {
	password, err := promptPassword("  Hasło do odszyfrowania kopii: ")
	if err != nil {
		return err
	}

	if err := backup.Restore(dataDir, backupFile, password); err != nil {
		return err
	}
	PrintSuccess("Baza danych została przywrócona z kopii zapasowej.")
	PrintSuccess("Poprzednia baza zapisana jako gobank.db.bak")
	return nil
}

// --- Helpers ---

// withSession loads the session file and calls fn only if a user is logged in.
func withSession(sessionPath string, fn func(*session) error) error {
	data, err := os.ReadFile(sessionPath)
	if os.IsNotExist(err) {
		return errors.New("nie jestes zalogowany — uzyj: gobank login")
	}
	if err != nil {
		return fmt.Errorf("blad odczytu sesji: %w", err)
	}

	var s session
	if err := json.Unmarshal(data, &s); err != nil {
		return errors.New("uszkodzona sesja — zaloguj sie ponownie")
	}
	return fn(&s)
}

// prompt prints a label and reads a single line from stdin.
func prompt(label string) (string, error) {
	fmt.Print(label)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return scanner.Text(), scanner.Err()
}

// promptPassword prints a label and reads a password without echoing characters.
func promptPassword(label string) (string, error) {
	fmt.Print(label)
	raw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("blad odczytu hasla: %w", err)
	}
	return string(raw), nil
}

// parsePLN converts a user-supplied PLN string (e.g. "10.50" or "10,50")
// to grosze using integer arithmetic only — no floating point.
func parsePLN(s string) (int, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", ".")
	parts := strings.SplitN(s, ".", 2)

	pln, err := strconv.Atoi(parts[0])
	if err != nil || pln < 0 {
		return 0, errors.New("nieprawidlowa kwota — przyklad: 10.50")
	}

	grosze := 0
	if len(parts) == 2 {
		dec := parts[1]
		switch len(dec) {
		case 0:
			grosze = 0
		case 1:
			grosze, err = strconv.Atoi(dec + "0")
		default:
			grosze, err = strconv.Atoi(dec[:2])
		}
		if err != nil {
			return 0, errors.New("nieprawidlowa kwota — przyklad: 10.50")
		}
	}

	return pln*100 + grosze, nil
}

// formatPLN converts grosze to a human-readable PLN string (e.g. "10,50 PLN").
func formatPLN(grosze int) string {
	return fmt.Sprintf("%d,%02d PLN", grosze/100, grosze%100)
}

// translateType maps English transaction types to Polish display names.
func translateType(t string) string {
	switch t {
	case "deposit":
		return "Wplata"
	case "withdrawal":
		return "Wyplata"
	case "transfer":
		return "Przelew"
	default:
		return t
	}
}

