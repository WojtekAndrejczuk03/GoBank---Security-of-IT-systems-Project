package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/wojtekandrejczuk/gobank/internal/accounts"
	"github.com/wojtekandrejczuk/gobank/internal/auth"
	"github.com/wojtekandrejczuk/gobank/internal/backup"
	"github.com/wojtekandrejczuk/gobank/internal/db"
	"github.com/wojtekandrejczuk/gobank/web"
)

type sessionData struct {
	UserID   int
	Username string
}

// Server holds the HTTP server state.
type Server struct {
	db       *db.DB
	dataDir  string
	mu       sync.RWMutex
	sessions map[string]sessionData
}

// New creates a new Server.
func New(database *db.DB, dataDir string) *Server {
	return &Server{
		db:       database,
		dataDir:  dataDir,
		sessions: make(map[string]sessionData),
	}
}

// Start registers all routes and begins listening on addr (e.g. ":8080").
func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleDashboard)
	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/register", s.handleRegister)
	mux.HandleFunc("/deposit", s.handleDeposit)
	mux.HandleFunc("/transfer", s.handleTransfer)
	mux.HandleFunc("/history", s.handleHistory)
	mux.HandleFunc("/backup", s.handleBackup)
	mux.HandleFunc("/restore", s.handleRestore)
	mux.HandleFunc("/logout", s.handleLogout)

	fmt.Printf("  Serwer uruchomiony → http://localhost%s\n\n", addr)
	return http.ListenAndServe(addr, mux)
}

// --- Session helpers ---

func (s *Server) getSession(r *http.Request) (sessionData, bool) {
	cookie, err := r.Cookie("token")
	if err != nil {
		return sessionData{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.sessions[cookie.Value]
	return data, ok
}

func (s *Server) setSession(w http.ResponseWriter, data sessionData) {
	token := make([]byte, 32)
	rand.Read(token)
	tokenStr := hex.EncodeToString(token)

	s.mu.Lock()
	s.sessions[tokenStr] = data
	s.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    tokenStr,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSession(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("token"); err == nil {
		s.mu.Lock()
		delete(s.sessions, cookie.Value)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "token", Value: "", Path: "/", MaxAge: -1})
}

// --- Template rendering ---

type pageData map[string]any

func (s *Server) render(w http.ResponseWriter, page string, data pageData) {
	funcMap := template.FuncMap{
		"formatPLN":     formatPLN,
		"translateType": translateType,
		"add":           func(a, b int) int { return a + b },
	}
	t, err := template.New("").Funcs(funcMap).ParseFS(web.Templates,
		"templates/base.html",
		"templates/"+page+".html",
	)
	if err != nil {
		http.Error(w, "Błąd szablonu: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, "Błąd renderowania: "+err.Error(), http.StatusInternalServerError)
	}
}

func base(sess sessionData, loggedIn bool) pageData {
	return pageData{"LoggedIn": loggedIn, "Username": sess.Username}
}

// --- Handlers ---

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	sess, ok := s.getSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	d := base(sess, true)
	acc, err := accounts.GetBalance(s.db, sess.UserID)
	if err != nil {
		d["Error"] = err.Error()
	} else {
		d["AccountNumber"] = acc.AccountNumber
		d["Balance"] = formatPLN(acc.Balance)
	}
	s.render(w, "dashboard", d)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.getSession(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	d := base(sessionData{}, false)
	if r.Method == http.MethodPost {
		r.ParseForm()
		user, err := auth.Login(s.db, r.FormValue("username"), r.FormValue("password"))
		if err != nil {
			d["Error"] = err.Error()
			s.render(w, "login", d)
			return
		}
		s.setSession(w, sessionData{UserID: user.ID, Username: user.Username})
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, "login", d)
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.getSession(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	d := base(sessionData{}, false)
	if r.Method == http.MethodPost {
		r.ParseForm()
		password := r.FormValue("password")
		if password != r.FormValue("confirm") {
			d["Error"] = "Hasła nie są zgodne."
			s.render(w, "register", d)
			return
		}
		if err := auth.Register(s.db, r.FormValue("username"), password); err != nil {
			d["Error"] = err.Error()
			s.render(w, "register", d)
			return
		}
		d["Success"] = "Konto zostało utworzone. Możesz się teraz zalogować."
		s.render(w, "login", d)
		return
	}
	s.render(w, "register", d)
}

func (s *Server) handleDeposit(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.getSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	d := base(sess, true)
	if r.Method == http.MethodPost {
		r.ParseForm()
		amount, err := parsePLN(r.FormValue("amount"))
		if err != nil {
			d["Error"] = err.Error()
			s.render(w, "deposit", d)
			return
		}
		if err := accounts.Deposit(s.db, sess.UserID, amount); err != nil {
			d["Error"] = err.Error()
			s.render(w, "deposit", d)
			return
		}
		d["Success"] = fmt.Sprintf("Wpłacono %s pomyślnie.", formatPLN(amount))
	}
	s.render(w, "deposit", d)
}

func (s *Server) handleTransfer(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.getSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	d := base(sess, true)
	if r.Method == http.MethodPost {
		r.ParseForm()
		target := strings.TrimSpace(r.FormValue("target"))
		amount, err := parsePLN(r.FormValue("amount"))
		if err != nil {
			d["Error"] = err.Error()
			s.render(w, "transfer", d)
			return
		}
		if err := accounts.Transfer(s.db, sess.UserID, target, amount); err != nil {
			d["Error"] = err.Error()
			s.render(w, "transfer", d)
			return
		}
		d["Success"] = fmt.Sprintf("Przelew %s na konto %s wykonany pomyślnie.", formatPLN(amount), target)
	}
	s.render(w, "transfer", d)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.getSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	d := base(sess, true)
	txns, err := accounts.History(s.db, sess.UserID, 20)
	if err != nil {
		d["Error"] = err.Error()
	} else {
		d["Transactions"] = txns
	}
	s.render(w, "history", d)
}

func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.getSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	d := base(sess, true)
	if r.Method == http.MethodPost {
		r.ParseForm()
		path, err := backup.Create(s.dataDir, r.FormValue("password"))
		if err != nil {
			d["Error"] = err.Error()
		} else {
			d["Success"] = "Kopia zapasowa zapisana: " + path
		}
	}
	s.render(w, "backup", d)
}

func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.getSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	d := base(sess, true)
	if r.Method == http.MethodPost {
		r.ParseForm()
		file := strings.TrimSpace(r.FormValue("file"))
		if err := backup.Restore(s.dataDir, file, r.FormValue("password")); err != nil {
			d["Error"] = err.Error()
		} else {
			d["Success"] = "Baza danych została przywrócona. Poprzednia baza zapisana jako gobank.db.bak"
		}
	}
	s.render(w, "backup", d)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.clearSession(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// --- Helpers ---

func formatPLN(grosze int) string {
	return fmt.Sprintf("%d,%02d PLN", grosze/100, grosze%100)
}

func translateType(t string) string {
	switch t {
	case "deposit":
		return "Wpłata"
	case "withdrawal":
		return "Wypłata"
	case "transfer":
		return "Przelew"
	default:
		return t
	}
}

func parsePLN(s string) (int, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	parts := strings.SplitN(s, ".", 2)
	pln, err := strconv.Atoi(parts[0])
	if err != nil || pln < 0 {
		return 0, errors.New("nieprawidłowa kwota — przykład: 10.50")
	}
	grosze := 0
	if len(parts) == 2 {
		dec := parts[1]
		switch len(dec) {
		case 0:
		case 1:
			grosze, err = strconv.Atoi(dec + "0")
		default:
			grosze, err = strconv.Atoi(dec[:2])
		}
		if err != nil {
			return 0, errors.New("nieprawidłowa kwota — przykład: 10.50")
		}
	}
	return pln*100 + grosze, nil
}
