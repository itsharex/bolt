package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/fhsinchy/bolt/internal/cli"
)

const version = "0.1.0-dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "add":
		runAdd(args)
	case "list", "ls":
		runList(args)
	case "status":
		runStatus(args)
	case "pause":
		runPause(args)
	case "resume":
		runResume(args)
	case "cancel", "rm":
		runCancel(args)
	case "refresh":
		runRefresh(args)
	case "version", "--version", "-v":
		fmt.Printf("bolt %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`bolt - fast segmented download manager

Usage:
  bolt add <url> [flags]     Add and start a download
  bolt list [flags]          List downloads
  bolt status <id>           Show download details
  bolt pause <id>            Pause a download
  bolt resume <id|all>       Resume a paused download
  bolt cancel <id> [flags]   Cancel and remove a download
  bolt refresh <id> <url>    Update URL for a failed download
  bolt version               Show version

Add flags:
  --dir <path>        Download directory
  --filename <name>   Override filename
  --segments <n>      Number of segments (1-32)
  --header <k:v>      Custom header (repeatable)
  --referer <url>     Referer URL
  --json              Output as JSON

List flags:
  --status <status>   Filter by status (queued/active/paused/completed/error)
  --json              Output as JSON

Cancel flags:
  --delete-file       Also delete the downloaded file
`)
}

func runAdd(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: bolt add <url> [flags]")
		os.Exit(1)
	}

	opts := cli.AddOptions{
		URL:     args[0],
		Headers: make(map[string]string),
	}

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--dir":
			i++
			if i < len(args) {
				opts.Dir = args[i]
			}
		case "--filename":
			i++
			if i < len(args) {
				opts.Filename = args[i]
			}
		case "--segments":
			i++
			if i < len(args) {
				var n int
				fmt.Sscanf(args[i], "%d", &n)
				opts.Segments = n
			}
		case "--header":
			i++
			if i < len(args) {
				parts := strings.SplitN(args[i], ":", 2)
				if len(parts) == 2 {
					opts.Headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
				}
			}
		case "--referer":
			i++
			if i < len(args) {
				opts.Referer = args[i]
			}
		case "--json":
			opts.JSON = true
		}
	}

	c, err := cli.New()
	if err != nil {
		fatal(err)
	}
	defer c.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle SIGINT for graceful pause
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down gracefully...")
		_ = c.Shutdown(ctx)
		cancel()
	}()

	if err := c.Add(ctx, opts); err != nil {
		if ctx.Err() != nil {
			// Graceful shutdown
			os.Exit(0)
		}
		fatal(err)
	}
}

func runList(args []string) {
	opts := cli.ListOptions{}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--status":
			i++
			if i < len(args) {
				opts.Status = args[i]
			}
		case "--json":
			opts.JSON = true
		}
	}

	c, err := cli.New()
	if err != nil {
		fatal(err)
	}
	defer c.Close()

	if err := c.List(context.Background(), opts); err != nil {
		fatal(err)
	}
}

func runStatus(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: bolt status <id>")
		os.Exit(1)
	}

	c, err := cli.New()
	if err != nil {
		fatal(err)
	}
	defer c.Close()

	if err := c.Status(context.Background(), args[0]); err != nil {
		fatal(err)
	}
}

func runPause(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: bolt pause <id>")
		os.Exit(1)
	}

	c, err := cli.New()
	if err != nil {
		fatal(err)
	}
	defer c.Close()

	if err := c.Pause(context.Background(), args[0]); err != nil {
		fatal(err)
	}
}

func runResume(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: bolt resume <id|all>")
		os.Exit(1)
	}

	c, err := cli.New()
	if err != nil {
		fatal(err)
	}
	defer c.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down gracefully...")
		_ = c.Shutdown(ctx)
		cancel()
	}()

	if err := c.Resume(ctx, args[0], true); err != nil {
		if ctx.Err() != nil {
			os.Exit(0)
		}
		fatal(err)
	}
}

func runCancel(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: bolt cancel <id> [--delete-file]")
		os.Exit(1)
	}

	id := args[0]
	deleteFile := false
	for _, arg := range args[1:] {
		if arg == "--delete-file" {
			deleteFile = true
		}
	}

	c, err := cli.New()
	if err != nil {
		fatal(err)
	}
	defer c.Close()

	if err := c.Cancel(context.Background(), id, deleteFile); err != nil {
		fatal(err)
	}
}

func runRefresh(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: bolt refresh <id> <new-url>")
		os.Exit(1)
	}

	c, err := cli.New()
	if err != nil {
		fatal(err)
	}
	defer c.Close()

	if err := c.Refresh(context.Background(), args[0], args[1]); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
