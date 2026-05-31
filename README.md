# GoBank — Security of IT Systems Project

A secure command-line banking application written in Go, built as a university project demonstrating key security concepts: password hashing, AES-256 encryption, and encrypted database backup/recovery.

---

## Features

- User registration and login with **bcrypt** password hashing (cost 12)
- Bank account management — balance, deposits, transfers
- Full transaction history
- **AES-256-GCM** encrypted database backups and one-command restore
- Balance stored in grosze (integer arithmetic, no float rounding errors)
- All transfers are atomic SQL transactions — no partial state

---

## Tech Stack

| Component  | Technology                      |
|------------|---------------------------------|
| Language   | Go 1.21+                        |
| Database   | SQLite (`modernc.org/sqlite`)   |
| Passwords  | bcrypt (`golang.org/x/crypto`)  |
| Encryption | AES-256-GCM (standard library)  |
| Interface  | CLI                             |

---

## Project Structure

```
gobank/
├── main.go                  # Entry point — opens DB, calls router
├── go.mod
├── data/                    # SQLite database and encrypted backups (gitignored)
├── cmd/
│   └── router.go            # CLI command dispatcher and all command handlers
└── internal/
    ├── db/db.go             # Database init and schema migrations
    ├── auth/auth.go         # Registration, login, bcrypt hashing
    ├── accounts/accounts.go # Balance, deposits, transfers, history
    ├── crypto/crypto.go     # AES-256-GCM encrypt/decrypt, key derivation
    └── backup/backup.go     # Encrypted backup creation and restore
```

---

## Setup

**Requirements:** Go 1.21 or newer

```bash
git clone https://github.com/WojtekAndrejczuk03/GoBank---Security-of-IT-systems-Project.git
cd GoBank---Security-of-IT-systems-Project
go build -o gobank .
```

The compiled binary is called `gobank`. The database is created automatically in `data/gobank.db` on first run.

---

## Usage

### Register a new user

```bash
./gobank register
```
```
Nazwa użytkownika: wojtek
Hasło (min. 8 znaków):
Potwierdź hasło:
Konto 'wojtek' zostało utworzone pomyślnie.
```

### Login

```bash
./gobank login
```
```
Nazwa użytkownika: wojtek
Hasło:
Zalogowano jako 'wojtek'.
```

### Check balance

```bash
./gobank balance
```
```
Numer konta : PL94827364019283746501234567
Saldo       : 0,00 PLN
```

### Deposit money

```bash
./gobank deposit
```
```
Kwota wpłaty (PLN): 500.00
Wpłacono 500,00 PLN.
```

### Transfer to another account

```bash
./gobank transfer
```
```
Numer konta odbiorcy: PL12345678901234567890123456
Kwota przelewu (PLN): 100.50
Przelew 100,50 PLN na konto PL12345678901234567890123456 wykonany pomyślnie.
```

### Transaction history

```bash
./gobank history
```
```
Lp.  Typ          Kwota          Data                 Opis
------------------------------------------------------------------------
1    Przelew      100,50 PLN     2026-05-31 20:15:42  Przelew bankowy
2    Wplata       500,00 PLN     2026-05-31 20:14:10  Wpłata środków
```

### Create encrypted backup

```bash
./gobank backup
```
```
Hasło do zaszyfrowania kopii:
Kopia zapasowa zapisana: data/backup_2026-05-31_20-16-00.enc
```

### Restore from backup

```bash
./gobank restore data/backup_2026-05-31_20-16-00.enc
```
```
Hasło do odszyfrowania kopii:
Baza danych została przywrócona z kopii zapasowej.
Poprzednia baza zapisana jako gobank.db.bak
```

### Logout

```bash
./gobank logout
```
```
Wylogowano pomyślnie.
```

---

## Security Design

| Threat | Mitigation |
|---|---|
| Weak passwords | bcrypt with cost factor 12 |
| Password leakage | Passwords never stored in plaintext; terminal echo disabled during input |
| Database theft | Backups encrypted with AES-256-GCM; backup key derived from master password via SHA-256 |
| Data tampering | GCM authentication tag — any modification to the backup file is detected |
| Float precision bugs | All monetary values stored as integers (grosze) |
| Partial transfer state | All transfers wrapped in SQL transactions with rollback on failure |

---

## Available Commands

| Command | Description |
|---|---|
| `register` | Create a new user account |
| `login` | Log in (session saved to `data/session.json`) |
| `logout` | End the current session |
| `balance` | Show account number and current balance |
| `deposit` | Deposit money into your account |
| `transfer` | Transfer money to another account number |
| `history` | Show the last 20 transactions |
| `backup` | Create an AES-256-GCM encrypted backup |
| `restore <file>` | Restore the database from a backup file |
