package gigops

// Intake planning is deliberately read-only. The conversation layer extracts
// structured facts; this deterministic layer checks them and proposes work.
// It never treats a draft, guessed account, or submitted event as published.
import (
    "errors"
    "fmt"
    "net/url"
    "sort"
    "strings"
    "time"
)

// GigIntake contains only public-safe facts. Commercial booking terms, private
// email bodies, raw media, credentials and person data must never be copied here.
type GigIntake struct {
    SchemaVersion       int    `json:"schema_version"`
    GigID               string `json:"gig_id,omitempty"` // Preserve across reschedules.
    Artist              string `json:"artist"`
    Title               string `json:"title"`
    BookingStatus       string `json:"booking_status"` // proposed, held, confirmed, cancelled
    ConfirmationSource  string `json:"confirmation_source,omitempty"` // owner, official, contract
    PerformanceRole     string `json:"performance_role"` // guest, organizer
    Venue               string `json:"venue"`
    City                string `json:"city"`
    CountryCode         string `json:"country_code"`
    Start               string `json:"start"` // RFC3339 including offset
    End                 string `json:"end"`   // RFC3339 including offset
    Timezone            string `json:"timezone"` // IANA timezone
    Admission           string `json:"admission"` // free, paid, unknown
    OfficialSourceURL   string `json:"official_source_url,omitempty"`
}

type CampaignStep struct {
    ID              string   `json:"id"`
    Execution       string   `json:"execution"` // automatic-draft, adapter-required, owner-policy, human-physical
    State           string   `json:"state"` // ready, needs-verification, needs-authorization, human-handoff, waiting, skipped
    DependsOn       []string `json:"depends_on,omitempty"`
    Next            string   `json:"next"`
}

type GigIntakePlan struct {
    SchemaVersion    int            `json:"schema_version"`
    ReadOnly         bool           `json:"read_only"`
    PublicWrites     int            `json:"public_writes"`
    Artist           string         `json:"artist"`
    GigID            string         `json:"gig_id,omitempty"`
    BookingStatus    string         `json:"booking_status"`
    BookingReady     bool           `json:"booking_ready"`
    Blockers         []string       `json:"blockers"`
    CandidateChannels []string      `json:"candidate_channels"`
    ExcludedChannels  []string      `json:"excluded_channels,omitempty"`
    Steps            []CampaignStep `json:"steps"`
}

func allowed(value string, options ...string) bool {
    for _, option := range options { if value == option { return true } }
    return false
}

func isHttpsURL(raw string) bool {
    u, err := url.Parse(raw)
    return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Fragment == ""
}

// PlanGigIntake turns structured, user-supplied facts into a safe workflow. It
// makes no network requests, and candidate platforms are NOT publication claims.
// A durable gig ID is intentionally not guessed; reconcile existing bookings
// before assigning one, so a rescheduled gig does not become a duplicate.
func PlanGigIntake(g GigIntake, now time.Time) (GigIntakePlan, error) {
    if g.SchemaVersion != 1 { return GigIntakePlan{}, errors.New("unsupported gig intake schema") }
    if strings.TrimSpace(g.Artist) != "Balkazar" { return GigIntakePlan{}, errors.New("only Balkazar has an approved intake tenant") }
    if !allowed(g.BookingStatus,"proposed","held","confirmed","cancelled") {
        return GigIntakePlan{}, errors.New("booking_status must be proposed, held, confirmed or cancelled")
    }
    if g.PerformanceRole != "" && !allowed(g.PerformanceRole,"guest","organizer") {
        return GigIntakePlan{}, errors.New("performance_role must be guest or organizer")
    }
    if g.Admission != "" && !allowed(g.Admission,"free","paid","unknown") {
        return GigIntakePlan{}, errors.New("invalid admission value")
    }
    p:=GigIntakePlan{SchemaVersion:1,ReadOnly:true,PublicWrites:0,Artist:g.Artist,
        GigID:g.GigID, BookingStatus:g.BookingStatus,Blockers:[]string{},CandidateChannels:[]string{},Steps:[]CampaignStep{}}

    // Cancelled gigs must not enter promotional scheduling, even if metadata is
    // incomplete; instead reconcile/cancel already published external listings.
    if g.BookingStatus == "cancelled" {
        p.Steps = []CampaignStep{
            {ID:"reconcile-existing-listings",Execution:"adapter-required",State:"needs-verification",Next:"Find existing published/draft listings and queued posts before changing anything"},
            {ID:"withdraw-approved-content",Execution:"owner-policy",State:"needs-authorization",DependsOn:[]string{"reconcile-existing-listings"},Next:"Withdraw or correct existing event pages/posts only under the owner-approved policy; verify outcomes"},
        }
        return p,nil
    }

    if strings.TrimSpace(g.GigID)=="" {p.Blockers=append(p.Blockers,"Reconcile existing gigs and assign persistent gig_id before any external write")}
    if strings.TrimSpace(g.Title)=="" {p.Blockers=append(p.Blockers,"Confirmed public event title missing")}
    if strings.TrimSpace(g.Venue)=="" || strings.TrimSpace(g.City)=="" || strings.TrimSpace(g.CountryCode)=="" {
        p.Blockers=append(p.Blockers,"Venue, city or country missing")
    }
    if g.PerformanceRole=="" {p.Blockers=append(p.Blockers,"Guest performer vs organizer role unverified")}
    if g.BookingStatus=="confirmed" && !allowed(g.ConfirmationSource,"owner","official","contract") {
        p.Blockers=append(p.Blockers,"Confirmed booking requires source owner, official or contract")
    }
    if g.OfficialSourceURL!="" && !isHttpsURL(g.OfficialSourceURL) {
        p.Blockers=append(p.Blockers,"Official source URL must be HTTPS and must be verified independently")
    }
    var start time.Time
    var end time.Time
    var startOK bool
    if g.Timezone == "" {p.Blockers=append(p.Blockers,"IANA timezone is missing")}
    loc, locErr := time.LoadLocation(g.Timezone)
    if g.Timezone!="" && locErr!=nil {p.Blockers=append(p.Blockers,"IANA timezone is invalid")}
    start, err := time.Parse(time.RFC3339, g.Start)
    if err!=nil {p.Blockers=append(p.Blockers,"RFC3339 show start time missing or invalid") } else { startOK=true }
    end, err = time.Parse(time.RFC3339, g.End)
    if err!=nil {p.Blockers=append(p.Blockers,"RFC3339 show end time missing or invalid") }
    if startOK && err==nil && !end.After(start) {p.Blockers=append(p.Blockers,"Show end must be after start")}
    if startOK && locErr==nil && g.Timezone!="" {
        _, actualOffset := start.Zone()
        _, zoneOffset := start.In(loc).Zone()
        if actualOffset != zoneOffset {p.Blockers=append(p.Blockers,"Start offset inconsistent with IANA timezone")}
    }
    if err==nil && locErr==nil && g.Timezone!="" {
        _, actualOffset := end.Zone()
        _, zoneOffset := end.In(loc).Zone()
        if actualOffset != zoneOffset {p.Blockers=append(p.Blockers,"End offset inconsistent with IANA timezone")}
    }
    if startOK && start.Before(now) {p.Blockers=append(p.Blockers,"Show already started; reconcile historical show and skip advance promotions")}

    // Keep a stable, reviewable channel shortlist. No platform is contacted.
    p.CandidateChannels=[]string{"owned-website","facebook-event","bandsintown","songkick","metricool-social","follow-hub"}
    if strings.EqualFold(g.CountryCode,"TW") {
        p.CandidateChannels=append(p.CandidateChannels,"streetvoice","cultureexpress","taipei-travel-eligibility","musico-editorial","roam-taipei","taiwan-whats-on")
    }
    if g.PerformanceRole=="organizer" {
        p.CandidateChannels=append(p.CandidateChannels,"kktix-if-registration-needed","accupass-if-registration-needed")
    } else {
        p.ExcludedChannels=[]string{"kktix-own-event","accupass-own-event"}
    }
    sort.Strings(p.CandidateChannels)
    safeToPrepare := g.BookingStatus == "confirmed" && len(p.Blockers)==0
    p.BookingReady=safeToPrepare
    privateState:="ready"
    listingState:="needs-authorization"
    if !safeToPrepare { listingState="needs-verification" }
    if g.BookingStatus!="confirmed" { privateState="needs-verification" }
    p.Steps=[]CampaignStep{
        {ID:"reconcile-gig-identity",Execution:"automatic-draft",State:"ready",Next:"Compare artist/date/venue with live records; preserve existing gig_id on reschedule"},
        {ID:"validate-booking",Execution:"automatic-draft",State:privateState,DependsOn:[]string{"reconcile-gig-identity"},Next:"Verify source evidence, timing, role, venue and audience segments"},
        {ID:"curate-media",Execution:"automatic-draft",State:privateState,DependsOn:[]string{"validate-booking"},Next:"Select real archive candidates; require individual asset rights and privacy review before public use"},
        {ID:"prepare-bilingual-assets",Execution:"automatic-draft",State:privateState,DependsOn:[]string{"curate-media"},Next:"Draft EN/zh-Hant event cover, description, images, reels and platform variants; do not invent footage"},
        {ID:"prepare-owned-event-page",Execution:"automatic-draft",State:privateState,DependsOn:[]string{"validate-booking"},Next:"Propose website page + Event JSON-LD PR and exact show facts"},
        {ID:"publish-event-listings",Execution:"adapter-required",State:listingState,DependsOn:[]string{"prepare-owned-event-page","prepare-bilingual-assets"},Next:"Per eligible platform verify existing/draft listings, terms, account permission and scoped publication approval; record public URLs"},
        {ID:"schedule-social-promotion",Execution:"adapter-required",State:listingState,DependsOn:[]string{"prepare-bilingual-assets"},Next:"Plan segment-specific posts via existing Metricool MCP, no duplicate posts or extra subscriptions"},
        {ID:"fan-follow-and-stage-pack",Execution:"automatic-draft",State:privateState,DependsOn:[]string{"validate-booking"},Next:"Use existing /follow/ + LINE and QR art; request physical banner/scan verification separately"},
        {ID:"stage-and-recording",Execution:"human-physical",State:"human-handoff",DependsOn:[]string{"validate-booking"},Next:"Supply a reliable show runbook; on-site phones/soundboard, venue consent, people and clothing require humans"},
        {ID:"post-show-media-and-results",Execution:"automatic-draft",State:"waiting",DependsOn:[]string{"stage-and-recording"},Next:"After verified media ingest: archive originals, produce rights-cleared recap drafts and measure attributable results"},
    }
    return p,nil
}

func (p GigIntakePlan) String() string {return fmt.Sprintf("%s %s: %d steps, ready=%v",p.Artist,p.GigID,len(p.Steps),p.BookingReady)}
