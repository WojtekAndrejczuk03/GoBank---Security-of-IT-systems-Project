package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/wojtekandrejczuk/gobank/internal/accounts"
)

// --- Colour palette ---

var (
	cBlue   = lipgloss.Color("39")
	cGreen  = lipgloss.Color("82")
	cRed    = lipgloss.Color("196")
	cYellow = lipgloss.Color("220")
	cGray   = lipgloss.Color("244")
	cWhite  = lipgloss.Color("255")
	cDim    = lipgloss.Color("238")
)

// --- Base styles ---

var (
	sBold = lipgloss.NewStyle().Bold(true)

	sSuccess = lipgloss.NewStyle().Foreground(cGreen).Bold(true)
	sError   = lipgloss.NewStyle().Foreground(cRed).Bold(true)
	sLabel   = lipgloss.NewStyle().Foreground(cGray)
	sAmount  = lipgloss.NewStyle().Foreground(cYellow).Bold(true)
	sAccent  = lipgloss.NewStyle().Foreground(cBlue).Bold(true)
	sDim     = lipgloss.NewStyle().Foreground(cDim)

	sBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cBlue).
		Padding(1, 3)

	sTableHeader = lipgloss.NewStyle().
			Foreground(cBlue).
			Bold(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(cDim)
)

// --- Rendered components ---

// PrintBanner prints the GoBank ASCII banner.
func PrintBanner() {
	banner := lipgloss.NewStyle().
		Foreground(cBlue).
		Bold(true).
		Render(`
  ██████╗  ██████╗ ██████╗  █████╗ ███╗   ██╗██╗  ██╗
 ██╔════╝ ██╔═══██╗██╔══██╗██╔══██╗████╗  ██║██║ ██╔╝
 ██║  ███╗██║   ██║██████╔╝███████║██╔██╗ ██║█████╔╝
 ██║   ██║██║   ██║██╔══██╗██╔══██║██║╚██╗██║██╔═██╗
 ╚██████╔╝╚██████╔╝██████╔╝██║  ██║██║ ╚████║██║  ██╗
  ╚═════╝  ╚═════╝ ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═══╝╚═╝  ╚═╝`)

	subtitle := sDim.Render("  Bezpieczna aplikacja bankowa · Security of IT Systems\n")

	fmt.Println(banner)
	fmt.Println(subtitle)
}

// PrintSuccess prints a styled success message.
func PrintSuccess(msg string) {
	fmt.Println(sSuccess.Render("  ✓ " + msg))
}

// PrintError prints a styled error message.
func PrintError(msg string) {
	fmt.Println(sError.Render("  ✗ " + msg))
}

// RenderBalance renders a styled box with account info and balance.
func RenderBalance(acc *accounts.Account) string {
	accountLine := sLabel.Render("Numer konta") + "\n" +
		lipgloss.NewStyle().Foreground(cWhite).Render(acc.AccountNumber)

	divider := sDim.Render(strings.Repeat("─", 38))

	balanceLine := sLabel.Render("Saldo") + "\n" +
		sAmount.Render(formatPLN(acc.Balance))

	content := accountLine + "\n\n" + divider + "\n\n" + balanceLine

	return sBox.Render(content)
}

// RenderHistory renders a styled transaction history table.
func RenderHistory(txns []accounts.Transaction) string {
	if len(txns) == 0 {
		return sDim.Render("  Brak historii transakcji.")
	}

	header := sTableHeader.Render(
		fmt.Sprintf("  %-4s  %-10s  %-14s  %-19s  %s",
			"Lp.", "Typ", "Kwota", "Data", "Opis"),
	)

	var rows strings.Builder
	for i, t := range txns {
		typeStyled := typeColor(t.Type).Render(fmt.Sprintf("%-10s", translateType(t.Type)))
		amountStyled := sAmount.Render(fmt.Sprintf("%-14s", formatPLN(t.Amount)))
		dateStyled := sDim.Render(fmt.Sprintf("%-19s", t.CreatedAt))
		descStyled := lipgloss.NewStyle().Foreground(cWhite).Render(t.Description)

		rows.WriteString(fmt.Sprintf("  %-4d  %s  %s  %s  %s\n",
			i+1, typeStyled, amountStyled, dateStyled, descStyled))
	}

	return header + "\n" + rows.String()
}

// RenderHelp renders the styled help menu.
func RenderHelp() string {
	title := sAccent.Render("GoBank") + sDim.Render(" — bezpieczna aplikacja bankowa\n")

	type cmd struct {
		name string
		desc string
	}
	cmds := []cmd{
		{"register", "Rejestracja nowego użytkownika"},
		{"login", "Logowanie"},
		{"logout", "Wylogowanie"},
		{"balance", "Sprawdź saldo"},
		{"deposit", "Wpłać środki"},
		{"transfer", "Wykonaj przelew"},
		{"history", "Historia transakcji (ostatnie 20)"},
		{"backup", "Utwórz zaszyfrowaną kopię zapasową"},
		{"restore <plik>", "Przywróć bazę z kopii zapasowej"},
		{"web [port]", "Uruchom interfejs webowy (domyślnie :8080)"},
	}

	var sb strings.Builder
	sb.WriteString(title + "\n")
	sb.WriteString(sLabel.Render("  Użycie: ") + sBold.Render("gobank <komenda>") + "\n\n")
	for _, c := range cmds {
		sb.WriteString(fmt.Sprintf("  %s  %s\n",
			sAccent.Render(fmt.Sprintf("%-20s", c.name)),
			sLabel.Render(c.desc),
		))
	}
	return sb.String()
}

// typeColor picks a colour based on transaction type.
func typeColor(t string) lipgloss.Style {
	switch t {
	case "deposit":
		return lipgloss.NewStyle().Foreground(cGreen)
	case "withdrawal":
		return lipgloss.NewStyle().Foreground(cRed)
	default:
		return lipgloss.NewStyle().Foreground(cBlue)
	}
}
