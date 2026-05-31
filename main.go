package main

import (
	"fmt"
	"os"

	"github.com/wojtekandrejczuk/gobank/cmd"
	"github.com/wojtekandrejczuk/gobank/internal/db"
)

const dataDir = "data"

func main() {
	cmd.PrintBanner()

	database, err := db.Open(dataDir)
	if err != nil {
		cmd.PrintError(fmt.Sprintf("Błąd bazy danych: %v", err))
		os.Exit(1)
	}
	defer database.Close()

	if err := cmd.Run(database, dataDir, os.Args[1:]); err != nil {
		fmt.Println()
		cmd.PrintError(err.Error())
		fmt.Println()
		os.Exit(1)
	}
}
