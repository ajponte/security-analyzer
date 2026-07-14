import (
	"context"
	"fmt"
	"logslog"
	"os"
)

const (
	DEFAULT_LOG_LEVEL = slog.LevelDebug
)

func main() {
	// Create the logger instance.
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	fmt.Print("hello world")
}
