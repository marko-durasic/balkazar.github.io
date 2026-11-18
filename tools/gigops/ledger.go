package gigops

import (
  "encoding/json"
  "errors"
  "fmt"
  "net/url"
  "os"
  "path/filepath"
  "regexp"
  "strings"
  "time"
)

type Listing struct {
  State string `json:"state"`
  PublicURL string `json:"public_url,omitempty"`
  Fingerprint string `json:"fingerprint"`
  VerifiedAt string `json:"verified_at"`
}
type Ledger struct {
  SchemaVersion int `json:"schema_version"`
  Events map[string]map[string]Listing `json:"events"`
}

func NewLedger() Ledger { return Ledger{SchemaVersion:1, Events:make(map[string]map[string]Listing)} }
func DefaultLedgerPath() (string,error) {
  base,err:=os.UserConfigDir(); if err!=nil{return "",err}
  return filepath.Join(base,"balkazar-musicops","ledger.json"),nil
}
func LoadLedger(path string)(Ledger,error){
  f,err:=os.Open(path)
  if os.IsNotExist(err){return NewLedger(),nil}
  if err!=nil{return Ledger{},err}
  defer f.Close()
  var l Ledger
  if err:=json.NewDecoder(f).Decode(&l);err!=nil{return Ledger{},err}
  if l.SchemaVersion!=1{return Ledger{},errors.New("unsupported ledger version")}
  if l.Events==nil{l.Events=make(map[string]map[string]Listing)}
  return l,nil
}
var (
  songkickURL=regexp.MustCompile(`^/concerts/[1-9][0-9]*(?:-[^/]+)?/?$`)
  bandsURL=regexp.MustCompile(`^/e/[1-9][0-9]*(?:-[^/]+)?/?$`)
)
func ValidPublicListingURL(provider,raw string) bool {
  u,err:=url.Parse(raw)
  if err!=nil||u.Scheme!="https"||u.User!=nil||u.Port()!=""||u.RawQuery!=""||u.Fragment!=""{return false}
  host:=strings.ToLower(u.Hostname())
  switch provider {
    case "songkick":return (host=="songkick.com"||host=="www.songkick.com")&&songkickURL.MatchString(u.Path)
    case "bandsintown":return (host=="bandsintown.com"||host=="www.bandsintown.com")&&bandsURL.MatchString(u.Path)
    default:return false
  }
}

// RecordLive writes only to the PRIVATE LOCAL ledger. It never publishes anything.
// Explicit verification is required even when the URL came from another source.
func RecordLive(l *Ledger,b ReviewBundle,source,provider,publicURL string,confirmed bool,at time.Time)error{
  if !confirmed{return errors.New("explicit --confirmed-human-verification is required")}
  if !ValidPublicListingURL(provider,publicURL){return fmt.Errorf("unexpected %s public listing URL",provider)}
  var found *ReviewEvent
  for i:=range b.Events{if b.Events[i].URL==source{found=&b.Events[i];break}}
  if found==nil{return errors.New("source event not found in reviewed bundle")}
  if !fingerprintRE.MatchString(found.Fingerprint){return errors.New("missing source fingerprint")}
  if l.Events==nil{l.Events=make(map[string]map[string]Listing)}
  if l.Events[source]==nil{l.Events[source]=make(map[string]Listing)}
  current,exists:=l.Events[source][provider]
  if exists&&current.State=="live"{
    if current.PublicURL!=publicURL{return errors.New("conflicting live listing URL: check for duplicate before editing ledger")}
    if current.Fingerprint!=found.Fingerprint{return errors.New("source changed: review existing listing first")}
    return nil
  }
  l.Events[source][provider]=Listing{State:"live",PublicURL:publicURL,Fingerprint:found.Fingerprint,VerifiedAt:at.UTC().Format(time.RFC3339)}
  return nil
}
func SaveLedger(path string,l Ledger)error{
  if !filepath.IsAbs(path){return errors.New("ledger path must be absolute")}
  // Keep ledger out of any git repository (especially this public website).
  for parent:=filepath.Dir(path);;parent=filepath.Dir(parent){
    if _,err:=os.Stat(filepath.Join(parent,".git"));err==nil{return errors.New("refusing to store ledger inside a git repository")}
    if parent==filepath.Dir(parent){break}
  }
  if l.SchemaVersion!=1{return errors.New("invalid ledger version")}
  dir:=filepath.Dir(path)
  if err:=os.MkdirAll(dir,0700);err!=nil{return err}
  f,err:=os.CreateTemp(dir,".ledger-*.tmp");if err!=nil{return err}
  temp:=f.Name();defer os.Remove(temp)
  if err:=f.Chmod(0600);err!=nil{f.Close();return err}
  encoder:=json.NewEncoder(f);encoder.SetIndent("","  ")
  if err:=encoder.Encode(l);err!=nil{f.Close();return err}
  if err:=f.Sync();err!=nil{f.Close();return err}
  if err:=f.Close();err!=nil{return err}
  return os.Rename(temp,path)
}
