package main

// Private cloud-only GigOps intake for an already-authenticated connector
// session. This CLI has NO network listener, provider API or public writes.
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
	dir := flag.String("state-dir", "", "Absolute private cloud MusicOps state directory")
	file := flag.String("file", "", "Absolute JSON candidate under state-dir/inbox")
	apply := flag.Bool("apply", false, "Persist canonical gig privately and refresh queue; does NOT execute any jobs")
	flag.Parse()
	if *dir == "" || *file == "" {
		return errors.New("state-dir and file are required")
	}
	req, err := gigops.LoadVersionedCloudGigCandidate(*dir, *file)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if !*apply {
		preview, err := gigops.PreviewCloudGigIntake(req.Gig, time.Now())
		if err != nil {
			return err
		}
		return enc.Encode(preview)
	}
	result, err := gigops.ApplyVersionedCloudGigIntake(*dir, req, time.Now())
	// If queue materialization fails after campaign commit, retain the
	// truthful partial receipt; the timer can reconcile safely later.
	if result.SchemaVersion != 0 {
		if outputErr := enc.Encode(result); outputErr != nil && err == nil {
			err = outputErr
		}
	}
	return err
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "cloud intake:", err)
		os.Exit(2)
	}
}
