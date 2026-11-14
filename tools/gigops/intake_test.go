package gigops

import (
    "strings"
    "testing"
    "time"
)

func sampleGig() GigIntake {
    return GigIntake{
        SchemaVersion:1, GigID:"balkazar-european-winter-2026",Artist:"Balkazar",
        Title:"Balkazar at European Christmas Market",BookingStatus:"confirmed",ConfirmationSource:"owner",
        PerformanceRole:"guest",Venue:"Xiangti Avenue Plaza",City:"Taipei",CountryCode:"TW",
        Start:"2026-12-03T20:00:00+08:00",End:"2026-12-03T21:30:00+08:00",Timezone:"Asia/Taipei",
        Admission:"free",OfficialSourceURL:"https://balkazar.org/events/2026-12-03-european-christmas-market/",
    }
}
func early() time.Time {return time.Date(2026,11,14,0,0,0,0,time.UTC)}
func has(sl []string,q string)bool{for _,s:=range sl{if s==q{return true}};return false}
func step(p GigIntakePlan,id string)(CampaignStep,bool){for _,s:=range p.Steps{if s.ID==id{return s,true}};return CampaignStep{},false}
func TestConfirmedGuestPlanNeverWrites(t *testing.T){
    p,err:=PlanGigIntake(sampleGig(),early());if err!=nil{t.Fatal(err)}
    if !p.ReadOnly||p.PublicWrites!=0||!p.BookingReady||len(p.Steps)!=10 {t.Fatalf("unsafe or invalid plan %+v",p)}
    if !has(p.CandidateChannels,"streetvoice")||has(p.CandidateChannels,"kktix-own-event") {t.Fatal("guest eligibility incorrect")}
    if !has(p.ExcludedChannels,"kktix-own-event") {t.Fatal("ticketing was not excluded")}
    if s,ok:=step(p,"publish-event-listings");!ok||s.State!="needs-authorization" {t.Fatalf("public publish not gated %+v",s)}
    if s,ok:=step(p,"stage-and-recording");!ok||s.Execution!="human-physical" {t.Fatal("pretended camera work can be automated")}
}
func TestTentativeCannotPublish(t *testing.T){
    g:=sampleGig();g.BookingStatus="held";g.ConfirmationSource=""
    p,err:=PlanGigIntake(g,early());if err!=nil{t.Fatal(err)}
    if p.BookingReady {t.Fatal("held event treated as booked")}
    s,_:=step(p,"publish-event-listings");if s.State!="needs-verification" {t.Fatal("held gig could publish")}
}
func TestMissingFactsFailClosed(t *testing.T){
    g:=sampleGig();g.GigID="";g.Venue="";g.ConfirmationSource=""
    p,err:=PlanGigIntake(g,early());if err!=nil{t.Fatal(err)}
    if p.BookingReady||len(p.Blockers)<3 {t.Fatalf("missing facts accepted %+v",p)}
}
func TestCancellationStopsPromotion(t *testing.T){
    g:=sampleGig();g.BookingStatus="cancelled";g.Start="";g.End=""
    p,err:=PlanGigIntake(g,early());if err!=nil{t.Fatal(err)}
    if p.BookingReady||len(p.Steps)!=2||p.Steps[1].ID!="withdraw-approved-content" {t.Fatal("cancellation not safely gated")}
}
func TestOrganizerMayConsiderTicketingButNotPublish(t *testing.T){
    g:=sampleGig();g.PerformanceRole="organizer"
    p,err:=PlanGigIntake(g,early());if err!=nil{t.Fatal(err)}
    if !has(p.CandidateChannels,"kktix-if-registration-needed") {t.Fatal("missing organizer option")}
    s,_:=step(p,"publish-event-listings");if s.State!="needs-authorization" {t.Fatal("unapproved public action")}
}
func TestTemporalAndSourceGuard(t *testing.T){
    cases:=[]struct{label string; mutate func(*GigIntake); signal string}{
        {"offset",func(g *GigIntake){g.Start="2026-12-03T20:00:00Z"},"offset"},
        {"end before start",func(g *GigIntake){g.End="2026-12-03T19:30:00+08:00"},"after start"},
        {"past",func(g *GigIntake){g.Start="2025-12-03T20:00:00+08:00";g.End="2025-12-03T21:30:00+08:00"},"already started"},
        {"insecure source",func(g *GigIntake){g.OfficialSourceURL="http://example.org/"},"HTTPS"},
    }
    for _,c:=range cases{t.Run(c.label,func(t *testing.T){g:=sampleGig();c.mutate(&g);p,err:=PlanGigIntake(g,early());if err!=nil{t.Fatal(err)}
        if p.BookingReady {t.Fatal("invalid event marked ready")};if !strings.Contains(strings.Join(p.Blockers," "),c.signal){t.Fatalf("expected %q in %+v",c.signal,p.Blockers)}
    })}
}
func TestTenantAndEnumFailClosed(t *testing.T){
    g:=sampleGig();g.Artist="Fuzzy Wall";if _,err:=PlanGigIntake(g,early());err==nil{t.Fatal("cross-tenant accepted")}
    g=sampleGig();g.BookingStatus="published";if _,err:=PlanGigIntake(g,early());err==nil{t.Fatal("invalid status accepted")}
}
