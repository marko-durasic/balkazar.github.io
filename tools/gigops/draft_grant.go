package gigops

// Draft grants are issued by a trusted owner-controlled approval service.
// An AI-generated text/JSON declaration is NEVER evidence of approval.
// The agent sees only a signed grant and the owner's trusted public key.
import (
 "crypto/ed25519"
 "encoding/base64"
 "encoding/json"
 "errors"
 "fmt"
 "regexp"
 "sort"
 "strings"
 "time"
)

var grantIDRE=regexp.MustCompile(`^[a-z0-9][a-z0-9-]{7,79}$`)

type MetricoolDraftGrant struct {
 Version int `json:"version"`
 GrantID string `json:"grant_id"`
 Artist string `json:"artist"`
 BrandID string `json:"brand_id"`
 GigID string `json:"gig_id"` // Exact ID or "*" for bounded artist-wide draft authority
 Action string `json:"action"` // exactly "create-private-draft"; never schedule/publish
 Networks []string `json:"networks"`
 MediaURLAllowlist []string `json:"media_url_allowlist"`
 ValidFrom string `json:"valid_from"`
 ExpiresAt string `json:"expires_at"`
 MaxDrafts int `json:"max_drafts"` // enforced by trusted runner ledger, NOT a model counter
 SpendLimitNTD int `json:"spend_limit_ntd"` // must be zero
}
type SignedMetricoolDraftGrant struct {
 Grant MetricoolDraftGrant `json:"grant"`
 Signature string `json:"signature"` // Ed25519 over json.Marshal(Grant), base64 raw
}

// GrantTrust lives in a protected, operator-owned configuration namespace.
// Never let the model generate a signing key or supply its own trusted key.
type MetricoolGrantTrust struct {
 OwnerPublicKey ed25519.PublicKey
 RevokedGrantIDs map[string]bool
}

func isDraftNetwork(network string) bool {
 return allowed(network,"facebook","instagram","threads")
}
func validGrantMediaURL(raw string) bool {
 // First pilot only allows rights-reviewed public files owned by Balkazar.
 // Google Drive/other providers require a separate rights-checked asset adapter.
 return strings.HasPrefix(raw,"https://balkazar.org/assets/") &&
        isHttpsURL(raw) && !strings.Contains(raw,"?") && !strings.Contains(raw,"#")
}

func validStringSet(items []string, max int, validate func(string)bool) bool {
 if len(items)==0||len(items)>max{return false}
 seen:=map[string]bool{}
 for _,x:=range items {
   if !validate(x)||seen[x]{return false}
   seen[x]=true
 }
 return true
}
func uniqueSorted(items []string) []string {
 out:=append([]string(nil),items...)
 sort.Strings(out)
 return out
}
func VerifyMetricoolDraftGrant(s SignedMetricoolDraftGrant, trust MetricoolGrantTrust, now time.Time) (MetricoolDraftGrant,error) {
 g:=s.Grant
 if len(trust.OwnerPublicKey)!=ed25519.PublicKeySize {
   return MetricoolDraftGrant{},errors.New("no pinned owner public key: fail closed")
 }
 if trust.RevokedGrantIDs[g.GrantID] {
   return MetricoolDraftGrant{},errors.New("owner revoked this grant")
 }
 if g.Version!=1||!grantIDRE.MatchString(g.GrantID)||
   g.Artist!="Balkazar"||g.BrandID!=balkazarMetricoolBrandID||
   (g.GigID!="*"&&!durableGigID.MatchString(g.GigID))||
   g.Action!="create-private-draft"||g.SpendLimitNTD!=0||
   g.MaxDrafts<1||g.MaxDrafts>20{
   return MetricoolDraftGrant{},errors.New("grant has unsupported identity, action or limits")
 }
 if !validStringSet(g.Networks,3,isDraftNetwork) {
   return MetricoolDraftGrant{},errors.New("invalid grant networks")
 }
 if len(g.MediaURLAllowlist)>10{return MetricoolDraftGrant{},errors.New("too many approved media assets")}
 seenMedia:=map[string]bool{}
 for _,m:=range g.MediaURLAllowlist{
   if !validGrantMediaURL(m)||seenMedia[m]{
     return MetricoolDraftGrant{},errors.New("grant media URL must be a unique owned /assets path")
   }
   seenMedia[m]=true
 }
 start,err:=time.Parse(time.RFC3339,g.ValidFrom)
 if err!=nil{return MetricoolDraftGrant{},errors.New("invalid grant valid_from")}
 end,err:=time.Parse(time.RFC3339,g.ExpiresAt)
 if err!=nil{return MetricoolDraftGrant{},errors.New("invalid grant expires_at")}
 if !end.After(start)||end.Sub(start)>90*24*time.Hour||
    now.Before(start)||!now.Before(end) {
   return MetricoolDraftGrant{},errors.New("draft grant expired, not active or longer than 90 days")
 }
 signature,err:=base64.RawStdEncoding.DecodeString(s.Signature)
 if err!=nil||len(signature)!=ed25519.SignatureSize {
   return MetricoolDraftGrant{},errors.New("invalid signed grant encoding")
 }
 message,err:=json.Marshal(g)
 if err!=nil{return MetricoolDraftGrant{},fmt.Errorf("marshal grant: %w",err)}
 if !ed25519.Verify(trust.OwnerPublicKey,message,signature) {
   return MetricoolDraftGrant{},errors.New("grant signature not issued by pinned owner authority")
 }
 return g,nil
}

// GrantUsage describes receipts in the trusted executor's PRIVATE ledger.
// The current GitHub draft code cannot self-issue a grant or persist it.
type GrantUsage struct {
 GrantID string `json:"grant_id"`
 DraftUUIDs []string `json:"draft_uuids"`
 InFlightKeys []string `json:"in_flight_keys"`
}
func (u GrantUsage)MayReserve(g MetricoolDraftGrant, intentKey string)error {
 if u.GrantID!=g.GrantID{return errors.New("missing trusted grant usage record")}
 seen:=map[string]bool{}
 for _,id:=range u.DraftUUIDs{
   if !metricoolUUIDRE.MatchString(id)||seen[id] {return errors.New("corrupted draft receipt ledger")}
   seen[id]=true
 }
 keys:=map[string]bool{}
 for _,k:=range u.InFlightKeys{
   if !storedFingerprintRE.MatchString(k)||keys[k] {return errors.New("corrupted in-flight key ledger")}
   keys[k]=true
 }
 if keys[intentKey]{return errors.New("draft intent already reserved; reconcile provider state before retry")}
 if len(u.DraftUUIDs)+len(u.InFlightKeys)>=g.MaxDrafts {
   return errors.New("grant draft capacity exhausted")
 }
 return nil
}
