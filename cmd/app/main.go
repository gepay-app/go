package main

import (
	"log/slog"
	"os"

	"app/internal/bootstrap"
)

func main() {
	if err := bootstrap.Run(); err != nil {
		slog.Error("app stopped", "error", err)
		os.Exit(1)
	}
}
