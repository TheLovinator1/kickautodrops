package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"kickautodrops/internal/kick"
	"kickautodrops/internal/tui"
)

func main() {
	// Parse CLI flags
	modeStr := flag.String("mode", "streamer", "farming mode: streamer or general")
	categoryID := flag.Int("category", 13, "game category ID (13 = Rust)")
	flag.Parse()

	// Determine farming mode
	var mode tui.FarmingMode
	switch *modeStr {
	case "streamer":
		mode = tui.ModeStreamer
	case "general":
		mode = tui.ModeGeneral
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s (use 'streamer' or 'general')\n", *modeStr)
		os.Exit(1)
	}

	// Create cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Create the Kick HTTP client
	client := kick.NewClient()

	// Create log channel (buffered, 1024 lines)
	logCh := make(chan string, 1024)

	// Set up the log adapter to route output to the TUI
	tui.SetLogAdapter(logCh)

	// Create the TUI model
	model := tui.NewModel(logCh)

	// Create and start the bubbletea program
	p := tea.NewProgram(&model, tea.WithAltScreen())

	// Start the farming loop in a background goroutine
	tui.RunFarming(ctx, client, logCh, p, mode, *categoryID)

	// Run the TUI (blocks until quit)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
