package gigops

import (
 "crypto/ed25519"
 "encoding/base64"
 "encoding/json"
 "strings"
 "testing"
 "time"
)

func draftNow()time.Time {return time.Date(2026,11,14,13,30,0,0,time.FixedZone("TWT",8*3600))}
func ownerTestKey()(ed25519.PublicKey,ed25519.PrivateKey) {
 seed:=make([]byte,ed25519.SeedSize)
 for i:=range seed{seed[i]=byte(i+1)}
 priv:=ed25519.NewKeyFromSeed(seed)
 return priv.Public().(ed25519.PublicKey),priv
}
func approvedGrant(gigID string)MetricoolDraftGrant {
 n:=draftNow()
 return MetricoolDraftGrant{
  Version:1,GrantID:"owner-approved-grant-2026",Artist:"Balkazar",
  BrandID:balkazarMetricoolBrandID,GigID:gigID,Action:"create-private-draft",
  Networks:[]string{"facebook","instagram","threads"},
  MediaURLAllowlist:[]string{"https://balkazar.org/assets/images/gallery/g-christmas-market-horns.jpg"},
  ValidFrom:n.Add(-time.Hour).Format(time.RFC3339),
  ExpiresAt:n.Add(20*24*time.Hour).Format(time.RFC3339),
  MaxDrafts:3,SpendLimitNTD:0,
 }
}
func signTestGrant(g MetricoolDraftGrant,privateKey ed25519.PrivateKey) SignedMetricoolDraftGrant {
 bytes,_:=json.Marshal(g)
 return SignedMetricoolDraftGrant{Grant:g,Signature:base64.RawStdEncoding.EncodeToString(ed25519.Sign(privateKey,bytes))}
}
func draftFixture(campaign StoredCampaign)MetricoolDraftCandidate{
 return MetricoolDraftCandidate{
  GigID:campaign.Gig.GigID,SourceRevision:campaign.Revision,
  PublicationAt:"2026-11-24T19:00:00+08:00",
  Networks:[]string{"threads","facebook","instagram"},
  Text:"Balkazar is back at the European Christmas Market! Free show, December 3. Come hear Balkan music and ska in Taipei: "+campaign.Gig.OfficialSourceURL,
  Media:[]string{"https://balkazar.org/assets/images/gallery/g-christmas-market-horns.jpg"},
 }
}
func draftTestSetup(t *testing.T)(CampaignStore,MetricoolDraftCandidate,SignedMetricoolDraftGrant,MetricoolGrantTrust,GrantUsage){
 t.Helper()
 s:=NewCampaignStore()
 g:=sampleGig()
 if _,err:=s.ReconcileGig(g,draftNow());err!=nil{t.Fatal(err)}
 pub,priv:=ownerTestKey()
 grant:=approvedGrant(g.GigID)
 signed:=signTestGrant(grant,priv)
 return s,draftFixture(s.Campaigns[g.GigID]),signed,
 MetricoolGrantTrust{OwnerPublicKey:pub,RevokedGrantIDs:map[string]bool{}},
 GrantUsage{GrantID:grant.GrantID,DraftUUIDs:[]string{},InFlightKeys:[]string{}}
}
func TestDraftGrantAuthenticatesOwner(t *testing.T){
 s,c,grant,trust,usage:=draftTestSetup(t)
 result,err:=BuildMetricoolDraftIntent(s,c,grant,trust,usage,nil,draftNow())
 if err!=nil{t.Fatal(err)}
 if !result.ReadOnly||result.PublicWrites!=0||result.ExecutionAllowed||
  result.Status!="needs-live-readback"||result.CreatePostArguments==nil{
  t.Fatalf("intent incorrectly marked executable: %+v",result)
 }
 if !storedFingerprintRE.MatchString(result.IntentKey){t.Fatal("missing stable intent fingerprint")}
 if result.CreatePostArguments.BlogID!=balkazarMetricoolBrandID{t.Fatal("wrong brand")}
 var body map[string]any
 if err:=json.Unmarshal([]byte(result.CreatePostArguments.Info),&body);err!=nil{t.Fatal(err)}
 if body["draft"]!=true||body["autoPublish"]!=false{t.Fatalf("payload could publish: %v",body)}
 providers:=body["providers"].([]any)
 if len(providers)!=3{t.Fatalf("missing networks: %+v",providers)}
 same,err:=BuildMetricoolDraftIntent(s,c,grant,trust,usage,nil,draftNow())
 if err!=nil||same.IntentKey!=result.IntentKey{t.Fatal("intent not deterministic")}
}
func TestNoSelfApprovalForgedAndRevokedGrants(t *testing.T){
 s,c,g,trust,usage:=draftTestSetup(t)
 pub,priv:=ownerTestKey()
 _=pub
 wrongPriv:=ed25519.NewKeyFromSeed(make([]byte,32))
 _,err:=BuildMetricoolDraftIntent(s,c,g,MetricoolGrantTrust{OwnerPublicKey:wrongPriv.Public().(ed25519.PublicKey)},usage,nil,draftNow())
 if err==nil{t.Fatal("forged signer accepted")}
 bad:=g
 bad.Grant.MaxDrafts=20 // Changes unsigned policy.
 if _,err:=BuildMetricoolDraftIntent(s,c,bad,trust,usage,nil,draftNow());err==nil{
  t.Fatal("model-edited grant accepted")
 }
 expired:=g.Grant
 expired.ExpiresAt=draftNow().Add(-time.Second).Format(time.RFC3339)
 signedExpired:=signTestGrant(expired,priv)
 if _,err:=BuildMetricoolDraftIntent(s,c,signedExpired,trust,usage,nil,draftNow());err==nil{
  t.Fatal("expired grant accepted")
 }
 trust.RevokedGrantIDs=map[string]bool{g.Grant.GrantID:true}
 if _,err:=BuildMetricoolDraftIntent(s,c,g,trust,usage,nil,draftNow());err==nil{
  t.Fatal("revoked grant accepted")
 }
 trust.OwnerPublicKey=nil
 if _,err:=BuildMetricoolDraftIntent(s,c,g,trust,usage,nil,draftNow());err==nil{
  t.Fatal("missing pinned owner key accepted")
 }
}
func TestDraftFailsOnPublicPostingOrWrongScope(t *testing.T){
 s,c,g,trust,usage:=draftTestSetup(t)
 _,priv:=ownerTestKey()
 changes:=[]struct{name string;modify func(*MetricoolDraftGrant)}{
  {"spent-money",func(v *MetricoolDraftGrant){v.SpendLimitNTD=1}},
  {"publish",func(v *MetricoolDraftGrant){v.Action="publish"}},
  {"wrong-brand",func(v *MetricoolDraftGrant){v.BrandID="99999"}},
  {"wrong-gig",func(v *MetricoolDraftGrant){v.GigID="some-other-gig"}},
  {"unapproved-network",func(v *MetricoolDraftGrant){v.Networks=[]string{"linkedin"}}},
  {"no-budget-cap",func(v *MetricoolDraftGrant){v.MaxDrafts=0}},
  {"duration-too-long",func(v *MetricoolDraftGrant){v.ExpiresAt=draftNow().Add(95*24*time.Hour).Format(time.RFC3339)}},
  {"unowned-media",func(v *MetricoolDraftGrant){v.MediaURLAllowlist=[]string{"https://random.example/photo.jpg"}}},
 }
 for _,test:=range changes {
  t.Run(test.name,func(t *testing.T){
   modified:=g.Grant
   test.modify(&modified)
   signed:=signTestGrant(modified,priv)
   if _,err:=BuildMetricoolDraftIntent(s,c,signed,trust,usage,nil,draftNow());err==nil{
    t.Fatal("unsafe signed scope accepted")
   }
  })
 }
 // Even a *valid* owner grant excludes channels not in its list.
 narrower:=g.Grant
 narrower.Networks=[]string{"facebook"}
 signed:=signTestGrant(narrower,priv)
 if _,err:=BuildMetricoolDraftIntent(s,c,signed,trust,usage,nil,draftNow());err==nil{
  t.Fatal("draft request bypassed network grant")
 }
}
func TestDraftMediaAndGuestTicketingGates(t *testing.T){
 s,c,g,trust,usage:=draftTestSetup(t)
 c.Media=nil
 if _,err:=BuildMetricoolDraftIntent(s,c,g,trust,usage,nil,draftNow());err==nil{
  t.Fatal("Instagram without media accepted")
 }
 c=draftFixture(s.Campaigns[c.GigID])
 c.Media=[]string{"https://example.com/crowd.jpg"}
 if _,err:=BuildMetricoolDraftIntent(s,c,g,trust,usage,nil,draftNow());err==nil{
  t.Fatal("unowned footage accepted")
 }
 c=draftFixture(s.Campaigns[c.GigID])
 c.Text="Unrelated event; not the canonical URL"
 if _,err:=BuildMetricoolDraftIntent(s,c,g,trust,usage,nil,draftNow());err==nil{
  t.Fatal("candidate without correct gig URL accepted")
 }
 c=draftFixture(s.Campaigns[c.GigID])
 c.SourceRevision++
 if _,err:=BuildMetricoolDraftIntent(s,c,g,trust,usage,nil,draftNow());err==nil{
  t.Fatal("stale campaign revision accepted")
 }
}
func TestDraftRecognizesExistingAndConflictingPosts(t *testing.T){
 s,c,g,trust,usage:=draftTestSetup(t)
 networks,_:=canonicalNetworks(c.Networks)
 matching:=MetricoolReadbackPreview{
  ReadOnly:true,PublicWrites:0,BrandID:balkazarMetricoolBrandID,
  GigID:c.GigID,EventURL:s.Campaigns[c.GigID].Gig.OfficialSourceURL,
  MatchedPosts:[]MetricoolPostObservation{{
   PostUUID:"existing-metricool-draft-123",State:"observed-draft",
   Networks:networks,ContentSHA256:contentDigest(c.Text,c.Media,networks),
   PublicationAt:c.PublicationAt,
  }},
 }
 result,err:=BuildMetricoolDraftIntent(s,c,g,trust,usage,&matching,draftNow())
 if err!=nil{t.Fatal(err)}
 if result.Status!="existing-draft"||result.ExistingUUID==""||result.CreatePostArguments!=nil{
  t.Fatalf("existing post not blocked from duplication: %+v",result)
 }
 matching.MatchedPosts[0].ContentSHA256=strings.Repeat("b",64)
 result,err=BuildMetricoolDraftIntent(s,c,g,trust,usage,&matching,draftNow())
 if err!=nil{t.Fatal(err)}
 if result.Status!="blocked-conflict"||result.CreatePostArguments!=nil{t.Fatal("same slot/network conflict not blocked")}
}
func TestDraftGrantUsageStopsReplays(t *testing.T){
 s,c,g,trust,usage:=draftTestSetup(t)
 first,err:=BuildMetricoolDraftIntent(s,c,g,trust,usage,nil,draftNow())
 if err!=nil{t.Fatal(err)}
 usage.InFlightKeys=[]string{first.IntentKey}
 if _,err:=BuildMetricoolDraftIntent(s,c,g,trust,usage,nil,draftNow());err==nil{
  t.Fatal("in-flight retry not blocked")
 }
 usage.InFlightKeys=nil
 usage.DraftUUIDs=[]string{"one-draft","two-draft","three-draft"}
 if _,err:=BuildMetricoolDraftIntent(s,c,g,trust,usage,nil,draftNow());err==nil{
  t.Fatal("capacity limit not enforced")
 }
}
func TestCancelledOrPastShowCannotPrepareDraft(t *testing.T){
 s,c,g,trust,usage:=draftTestSetup(t)
 original:=s.Campaigns[c.GigID]
 original.Gig.BookingStatus="cancelled"
 s.Campaigns[c.GigID]=original
 if _,err:=BuildMetricoolDraftIntent(s,c,g,trust,usage,nil,draftNow());err==nil{
  t.Fatal("cancelled show accepted for draft")
 }
 original.Gig.BookingStatus="confirmed"
 s.Campaigns[c.GigID]=original
 c.PublicationAt="2026-12-04T19:00:00+08:00"
 if _,err:=BuildMetricoolDraftIntent(s,c,g,trust,usage,nil,draftNow());err==nil{
  t.Fatal("pre-show campaign allowed after show")
 }
}
