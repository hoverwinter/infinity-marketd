package main

import (
	"context"
	"os"

	"github.com/hoverwinter/infinity-marketd/internal/infinitycli"
)

func main() {
	os.Exit(infinitycli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
