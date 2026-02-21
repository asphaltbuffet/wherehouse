// Package main is the entrypoint of the wherehouse application.
package main

import (
	"context"
	"os"

	"github.com/asphaltbuffet/wherehouse/cmd"
)

func main() {
	ctx := context.Background()

	// fang.Execute handles error output via DefaultErrorHandler
	// so we only need to set the exit code on error
	if err := cmd.Execute(ctx); err != nil {
		os.Exit(1)
	}
}
