package main

import (
	"context"
	"os"

	"github.com/hoverwinter/infinity-marketd/internal/consolecli"
)

func main() {
	os.Exit(consolecli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
