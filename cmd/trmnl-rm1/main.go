package main

import (
	"fmt"
	"os"

	"github.com/robinsandborg/rm1-trmnl/internal/trmnl"
)

func main() {
	app := trmnl.NewApp(os.Stdout, os.Stderr)
	if err := app.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
