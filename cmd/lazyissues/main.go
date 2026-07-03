package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"lazyissues/internal/issues"
	"lazyissues/internal/tui"
)

const defaultDBPath = "./.pi/issues.db"

var version = "dev"

type config struct {
	dbPath      string
	showVersion bool
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	cfg := config{}
	fs := flag.NewFlagSet("lazyissues", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&cfg.dbPath, "db", defaultDBPath, "path to local issues SQLite database")
	fs.BoolVar(&cfg.showVersion, "version", false, "print version information and exit")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s [--db PATH]\n\n", fs.Name())
		fmt.Fprintln(fs.Output(), "lazyissues is a terminal UI for browsing local pi issue queues.")
		fmt.Fprintln(fs.Output(), "\nOptions:")
		fs.PrintDefaults()
	}

	if hasHelpArg(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if cfg.showVersion {
		fmt.Fprintf(stdout, "lazyissues %s\n", version)
		return 0
	}

	repo, err := issues.Open(cfg.dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	defer repo.Close()

	loadedIssues, err := repo.List(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	model := tui.NewModel(loadedIssues, cfg.dbPath).WithIssueLoader(repo.List)
	if !canRunInteractive(stdout) {
		fmt.Fprint(stdout, model.WithSize(100, 30).View())
		return 0
	}

	program := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(stderr, "error: start TUI: %v\n", err)
		return 1
	}
	return 0
}

func hasHelpArg(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

func canRunInteractive(stdout io.Writer) bool {
	return stdout == os.Stdout && isCharDevice(os.Stdin) && isCharDevice(os.Stdout)
}

func isCharDevice(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
