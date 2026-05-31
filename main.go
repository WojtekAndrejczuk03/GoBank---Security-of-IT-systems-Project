package main

import (
	"fmt"
	"os"

	"github.com/wojtekandrejczuk/gobank/cmd"
	"github.com/wojtekandrejczuk/gobank/internal/db"
)

const dataDir = "data"

func main() {
	database, err := db.Open(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Blad: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := cmd.Run(database, dataDir, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Blad: %v\n", err)
		os.Exit(1)
	}
}
