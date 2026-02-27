package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/fhsinchy/bolt/internal/config"
	"github.com/fhsinchy/bolt/internal/db"
	"github.com/fhsinchy/bolt/internal/engine"
	"github.com/fhsinchy/bolt/internal/event"
	"github.com/fhsinchy/bolt/internal/model"
	"github.com/fhsinchy/bolt/internal/queue"
)

// CLI holds the components needed for CLI commands.
type CLI struct {
	store  *db.Store
	cfg    *config.Config
	bus    *event.Bus
	eng    *engine.Engine
	queue  *queue.Manager
	dbPath string
}

// New creates a new CLI instance, opening the database and loading config.
func New() (*CLI, error) {
	cfgPath := config.DefaultPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	dbPath := filepath.Join(config.Dir(), "bolt.db")
	store, err := db.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	bus := event.NewBus()
	eng := engine.New(store, cfg, bus)

	var queueMgr *queue.Manager
	queueMgr = queue.New(store, bus, cfg.MaxConcurrent, func(ctx context.Context, id string) error {
		return eng.StartDownload(ctx, id)
	})

	return &CLI{
		store:  store,
		cfg:    cfg,
		bus:    bus,
		eng:    eng,
		queue:  queueMgr,
		dbPath: dbPath,
	}, nil
}

// Close releases CLI resources.
func (c *CLI) Close() {
	c.store.Close()
}

// Add adds a download and starts it.
func (c *CLI) Add(ctx context.Context, opts AddOptions) error {
	req := model.AddRequest{
		URL:      opts.URL,
		Filename: opts.Filename,
		Dir:      opts.Dir,
		Segments: opts.Segments,
		Headers:  opts.Headers,
	}

	if opts.Referer != "" {
		if req.Headers == nil {
			req.Headers = make(map[string]string)
		}
		req.Headers["Referer"] = opts.Referer
	}

	dl, err := c.eng.AddDownload(ctx, req)
	if err != nil {
		return fmt.Errorf("adding download: %w", err)
	}

	if opts.JSON {
		return json.NewEncoder(os.Stdout).Encode(dl)
	}

	fmt.Printf("Added: %s\n", dl.Filename)
	fmt.Printf("  ID:       %s\n", dl.ID)
	fmt.Printf("  Size:     %s\n", model.FormatBytes(dl.TotalSize))
	fmt.Printf("  Segments: %d\n", dl.SegmentCount)
	fmt.Printf("  Dir:      %s\n", dl.Dir)
	fmt.Println()

	// Start the download
	if err := c.eng.StartDownload(ctx, dl.ID); err != nil {
		return fmt.Errorf("starting download: %w", err)
	}

	// Subscribe to events and show progress
	ch, subID := c.bus.Subscribe()
	defer c.bus.Unsubscribe(subID)

	for evt := range ch {
		switch e := evt.(type) {
		case event.Progress:
			if e.DownloadID == dl.ID {
				fmt.Print(formatProgressBar(e, dl.Filename))
			}
		case event.DownloadCompleted:
			if e.DownloadID == dl.ID {
				fmt.Print(formatCompleted(dl.Filename))
				return nil
			}
		case event.DownloadFailed:
			if e.DownloadID == dl.ID {
				fmt.Print(formatFailed(dl.ID, e.Error))
				return fmt.Errorf("download failed: %s", e.Error)
			}
		}
	}

	return nil
}

// List shows downloads matching the filter.
func (c *CLI) List(ctx context.Context, opts ListOptions) error {
	downloads, err := c.eng.ListDownloads(ctx, model.ListFilter{
		Status: opts.Status,
	})
	if err != nil {
		return fmt.Errorf("listing downloads: %w", err)
	}

	if opts.JSON {
		return json.NewEncoder(os.Stdout).Encode(downloads)
	}

	if len(downloads) == 0 {
		fmt.Println("No downloads found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tFILENAME\tSIZE\tPROGRESS\tSTATUS")
	for _, dl := range downloads {
		id := dl.ID
		if len(id) > 12 {
			id = id[:12]
		}
		filename := dl.Filename
		if len(filename) > 30 {
			filename = filename[:27] + "..."
		}

		var progress string
		if dl.TotalSize > 0 {
			pct := float64(dl.Downloaded) / float64(dl.TotalSize) * 100
			progress = fmt.Sprintf("%.0f%%", pct)
		} else {
			progress = model.FormatBytes(dl.Downloaded)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			id, filename, model.FormatBytes(dl.TotalSize), progress, dl.Status)
	}
	w.Flush()
	return nil
}

// Status shows detailed info for a single download.
func (c *CLI) Status(ctx context.Context, id string) error {
	dl, segments, err := c.eng.GetDownload(ctx, id)
	if err != nil {
		return err
	}

	fmt.Printf("ID:           %s\n", dl.ID)
	fmt.Printf("URL:          %s\n", dl.URL)
	fmt.Printf("Filename:     %s\n", dl.Filename)
	fmt.Printf("Directory:    %s\n", dl.Dir)
	fmt.Printf("Size:         %s\n", model.FormatBytes(dl.TotalSize))
	fmt.Printf("Downloaded:   %s\n", model.FormatBytes(dl.Downloaded))
	fmt.Printf("Status:       %s\n", dl.Status)
	fmt.Printf("Segments:     %d\n", dl.SegmentCount)
	fmt.Printf("Created:      %s\n", dl.CreatedAt.Format("2006-01-02 15:04:05"))
	if dl.CompletedAt != nil {
		fmt.Printf("Completed:    %s\n", dl.CompletedAt.Format("2006-01-02 15:04:05"))
	}
	if dl.Error != "" {
		fmt.Printf("Error:        %s\n", dl.Error)
	}

	if len(segments) > 0 {
		fmt.Println("\nSegments:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "  #\tRANGE\tDOWNLOADED\tDONE")
		for _, seg := range segments {
			size := seg.EndByte - seg.StartByte + 1
			var pct string
			if size > 0 {
				pct = fmt.Sprintf("%.0f%%", float64(seg.Downloaded)/float64(size)*100)
			}
			doneStr := "no"
			if seg.Done {
				doneStr = "yes"
			}
			fmt.Fprintf(w, "  %d\t%s-%s\t%s (%s)\t%s\n",
				seg.Index,
				model.FormatBytes(seg.StartByte),
				model.FormatBytes(seg.EndByte),
				model.FormatBytes(seg.Downloaded),
				pct,
				doneStr)
		}
		w.Flush()
	}

	return nil
}

// Pause pauses a download.
func (c *CLI) Pause(ctx context.Context, id string) error {
	if err := c.eng.PauseDownload(ctx, id); err != nil {
		return err
	}
	fmt.Printf("Paused: %s\n", id)
	return nil
}

// Resume resumes a download.
func (c *CLI) Resume(ctx context.Context, id string, showProgress bool) error {
	if id == "all" {
		return c.resumeAll(ctx)
	}

	dl, err := c.store.GetDownload(ctx, id)
	if err != nil {
		return err
	}

	if err := c.eng.ResumeDownload(ctx, id); err != nil {
		return err
	}

	if !showProgress {
		fmt.Printf("Resumed: %s\n", id)
		return nil
	}

	// Show progress
	ch, subID := c.bus.Subscribe()
	defer c.bus.Unsubscribe(subID)

	for evt := range ch {
		switch e := evt.(type) {
		case event.Progress:
			if e.DownloadID == id {
				fmt.Print(formatProgressBar(e, dl.Filename))
			}
		case event.DownloadCompleted:
			if e.DownloadID == id {
				fmt.Print(formatCompleted(dl.Filename))
				return nil
			}
		case event.DownloadFailed:
			if e.DownloadID == id {
				fmt.Print(formatFailed(id, e.Error))
				return fmt.Errorf("download failed: %s", e.Error)
			}
		}
	}

	return nil
}

func (c *CLI) resumeAll(ctx context.Context) error {
	downloads, err := c.store.ListDownloads(ctx, string(model.StatusPaused), 0, 0)
	if err != nil {
		return err
	}

	if len(downloads) == 0 {
		fmt.Println("No paused downloads to resume.")
		return nil
	}

	for _, dl := range downloads {
		if err := c.eng.ResumeDownload(ctx, dl.ID); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to resume %s: %v\n", dl.ID[:12], err)
			continue
		}
		fmt.Printf("Resumed: %s (%s)\n", dl.ID[:12], dl.Filename)
	}

	return nil
}

// Cancel cancels a download.
func (c *CLI) Cancel(ctx context.Context, id string, deleteFile bool) error {
	if err := c.eng.CancelDownload(ctx, id, deleteFile); err != nil {
		return err
	}
	fmt.Printf("Cancelled: %s\n", id)
	if deleteFile {
		fmt.Println("  File deleted.")
	}
	return nil
}

// Refresh refreshes the URL for a failed download.
func (c *CLI) Refresh(ctx context.Context, id, newURL string) error {
	if err := c.eng.RefreshURL(ctx, id, newURL); err != nil {
		return err
	}
	fmt.Printf("URL refreshed for %s. Use 'bolt resume %s' to continue.\n", id, id)
	return nil
}

// Shutdown gracefully shuts down the engine.
func (c *CLI) Shutdown(ctx context.Context) error {
	return c.eng.Shutdown(ctx)
}

// AddOptions holds flags for the add command.
type AddOptions struct {
	URL      string
	Dir      string
	Filename string
	Segments int
	Headers  map[string]string
	Referer  string
	JSON     bool
}

// ListOptions holds flags for the list command.
type ListOptions struct {
	Status string
	JSON   bool
}
