package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

type syncOptions struct {
	today     bool
	yesterday bool
	lastNDays int
	dryRun    bool
	cleanUp   bool
}

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "config" {
		if err := runConfigCmd(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	opts := parseSyncFlags(os.Args[1:])

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := cfg.validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := run(cfg, opts); err != nil {
		fmt.Fprintf(os.Stderr, "🛑️ Sync failed: %v\n", err)
		os.Exit(1)
	}
}

func parseSyncFlags(args []string) syncOptions {
	var opts syncOptions
	fs := flag.NewFlagSet("time-sync", flag.ExitOnError)
	fs.BoolVar(&opts.today, "today", false, "Sync only today's entries")
	fs.BoolVar(&opts.yesterday, "yesterday", false, "Sync only yesterday's entries")
	fs.IntVar(&opts.lastNDays, "last-n-days", 0, "Sync entries from the last N days (excluding today)")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "Print what would be synced without posting anything")
	fs.BoolVar(&opts.cleanUp, "clean-up", false, "Mark Clockify tasks as done when the linked YouTrack issue is resolved")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: time-sync [flags]\n       time-sync config <set|get|list> [args]\n\nFlags:\n")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)
	return opts
}

func run(cfg *config, opts syncOptions) error {
	clockify := newClockifyClient(cfg)
	youtrack := newYouTrackClient(cfg)

	if opts.cleanUp {
		fmt.Println("⚙️ Running in clean-up mode")
		return cleanUp(clockify, youtrack)
	}

	if opts.dryRun {
		fmt.Println("⚙️ Running in dry-run mode")
	}

	return syncEntries(clockify, youtrack, time.Now().UTC(), opts)
}
