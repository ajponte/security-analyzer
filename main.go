package main

import (
	"context"
	"log/slog"
	"os"
)

const (
	DEFAULT_LOG_LEVEL = slog.LevelDebug
)

func main() {
	// Create the logger instance.
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: DEFAULT_LOG_LEVEL,
	}))

	ctx := context.Background()
	logger.InfoContext(ctx, "hello world")
}
