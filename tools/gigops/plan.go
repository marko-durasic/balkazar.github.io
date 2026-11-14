package gigops

import (
  "encoding/json"
  "errors"
  "fmt"
  "net/url"
  "os"
  "regexp"
  "strings"
)

type ReviewBundle struct {
  SchemaVersion int `json:"schema_version"`
  ReadOnly bool `json:"read_only"`
  Events []ReviewEvent `json:"events"`
}
type ReviewEvent struct {
  URL string `json:"url"`
  Fingerprint string `json:"fingerprint"`
  Start string `json:"start"`
  End string `json:"end"`
  Venue string `json:"venue"`
  Status string `json:"status"`
  SongkickBlockers []string `json:"songkick_blockers"`
}
type PlatformPlan struct {
  Platform string `json:"platform"`
  State string `json:"state"`
  PublicURL string `json:"public_url,omitempty"`
  NextAction string `json:"next_action"`
  Blockers []string `json:"blockers,omitempty"`
}
type EventPlan struct {
  SourceURL string `json:"source_url"`
  Fingerprint string `json:"fingerprint"`
  Start string `json:"start"`
  End string `json:"end"`
  Venue string `json:"venue"`
  SourceStatus string `json:"source_status"`
  Platforms []PlatformPlan `json:"platforms"`
}

var fingerprintRE = regexp.MustCompile(`^[a-f0-9]{64}$`)

// ValidSourceURL accepts only exact canonical Balkazar gig pages, not arbitrary URLs.
func ValidSourceURL(raw string) bool {
  u, err := url.Parse(raw)
  return err == nil && u.Scheme == "https" && strings.EqualFold(u.Host, "balkazar.org") &&
    strings.HasPrefix(u.Path, "/events/") && strings.HasSuffix(u.Path, "/") &&
    u.RawQuery == "" && u.Fragment == "" && u.User == nil
}

func LoadReviewBundle(path string) (ReviewBundle, error) {
  f, err := os.Open(path)
  if err != nil { return ReviewBundle{}, err }
  defer f.Close()
  var b ReviewBundle
  if err := json.NewDecoder(f).Decode(&b); err != nil { return ReviewBundle{}, err }
  if b.SchemaVersion != 1 || !b.ReadOnly { return ReviewBundle{}, errors.New("unsupported or non-read-only bundle") }
  seen := make(map[string]bool)
  for _, e := range b.Events {
    if !ValidSourceURL(e.URL) { return ReviewBundle{}, fmt.Errorf("invalid source URL: %q", e.URL) }
    if seen[e.URL] { return ReviewBundle{}, fmt.Errorf("duplicate source URL: %s", e.URL) }
    seen[e.URL] = true
    if !fingerprintRE.MatchString(e.Fingerprint) { return ReviewBundle{}, fmt.Errorf("invalid fingerprint for %s", e.URL) }
  }
  return b, nil
}

// Plans never treats a generated review artifact as proof of an external post.
func Plans(b ReviewBundle, l Ledger) []EventPlan {
  result := make([]EventPlan, 0, len(b.Events))
  for _, e := range b.Events {
    if !ValidSourceURL(e.URL) || !fingerprintRE.MatchString(e.Fingerprint) { continue }
    p := EventPlan{SourceURL:e.URL, Fingerprint:e.Fingerprint, Start:e.Start, End:e.End, Venue:e.Venue, SourceStatus:e.Status}
    for _, provider := range []string{"bandsintown", "songkick"} {
      pp := PlatformPlan{Platform:provider, State:"unknown", NextAction:"Check managed artist, venue and existing published/draft event before submission"}
      if e.End == "" || e.Start == "" { pp.Blockers = append(pp.Blockers, "Source MusicEvent missing start/end time") }
      if provider == "songkick" {
        pp.Blockers = append(pp.Blockers, e.SongkickBlockers...)
        pp.Blockers = append(pp.Blockers, "Unattended Songkick automation requires platform permission; use supervised Tourbox flow")
      }
      if byProvider, ok := l.Events[e.URL]; ok {
        if existing, found := byProvider[provider]; found {
          pp.State = existing.State
          pp.PublicURL = existing.PublicURL
          if existing.Fingerprint != e.Fingerprint {
            pp.NextAction = "Source changed: review edits and existing listing before updating"
            pp.Blockers = append(pp.Blockers, "Source fingerprint changed since last record")
          } else if existing.State == "live" {
            pp.NextAction = "Recorded live: do not duplicate; verify public page as necessary"
            pp.Blockers = nil
          }
        }
      }
      p.Platforms = append(p.Platforms, pp)
    }
    result = append(result,p)
  }
  return result
}
