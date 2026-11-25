package main

// MusicOps cloud dry-run worker. No external publishing, OAuth access, network,
// generic remote command execution or AWS paid resource provisioning.
import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	gigops "balkazar.org/musicops"
)

func run() error {
	dir := flag.String("state-dir", "", "Absolute private cloud state dir")
	mode := flag.String("mode", "run", "run, health or queue (read-only)")
	maxAge := flag.Duration("max-age", 15*time.Minute, "Maximum permitted heartbeat age for health check")
	flag.Parse()
	if *dir == "" {
		return errors.New("--state-dir must be configured by the operator")
	}
	now := time.Now()
	var result any
	switch *mode {
	case "run":
		r, err := gigops.RunCloudDryRun(*dir, now)
		if err != nil {
			return err
		}
		result = r
	case "health":
		r, err := gigops.CheckCloudDryRunHealth(*dir, now, *maxAge)
		if err != nil {
			return err
		}
		result = r
	case "queue":
		items, err := gigops.CloudQueueSnapshot(*dir)
		if err != nil {
			return err
		}
		result = items
	default:
		return errors.New("unknown mode; supported: run, health, queue")
	}
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	return e.Encode(result)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "musicops dry-run error:", err)
		os.Exit(2)
	}
}
