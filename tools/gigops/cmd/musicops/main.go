package main

import (
  "encoding/json"
  "flag"
  "fmt"
  "os"
  "time"

  gigops "balkazar.org/musicops"
)
func main(){
  if len(os.Args)<2{usage()}
  switch os.Args[1]{
    case "plan":plan(os.Args[2:])
    case "record-live":record(os.Args[2:])
    default:usage()
  }
}
func usage(){fmt.Fprintln(os.Stderr,"musicops plan --bundle FILE [--ledger FILE] | musicops record-live --bundle FILE --source URL --platform songkick|bandsintown --listing-url URL --confirmed-human-verification");os.Exit(2)}
func must(err error){if err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(2)}}
func ledgerPath(custom string)string{if custom!=""{return custom};v,err:=gigops.DefaultLedgerPath();must(err);return v}
func load(bundlePath,customLedger string)(gigops.ReviewBundle,gigops.Ledger){
  if bundlePath==""{fmt.Fprintln(os.Stderr,"--bundle required");os.Exit(2)}
  b,err:=gigops.LoadReviewBundle(bundlePath);must(err)
  l,err:=gigops.LoadLedger(ledgerPath(customLedger));must(err)
  return b,l
}
func plan(args []string){
  f:=flag.NewFlagSet("plan",flag.ExitOnError)
  b:=f.String("bundle","","Read-only review-bundle.json")
  l:=f.String("ledger","","Private local ledger path")
  must(f.Parse(args))
  bundle,ledger:=load(*b,*l)
  writer:=json.NewEncoder(os.Stdout);writer.SetIndent("","  ");must(writer.Encode(gigops.Plans(bundle,ledger)))
}
func record(args []string){
  f:=flag.NewFlagSet("record-live",flag.ExitOnError)
  b:=f.String("bundle","","Read-only reviewed bundle")
  l:=f.String("ledger","","Private local ledger path")
  source:=f.String("source","","Canonical Balkazar event page URL")
  provider:=f.String("platform","","bandsintown or songkick")
  listing:=f.String("listing-url","","Human-verified live listing URL")
  confirmed:=f.Bool("confirmed-human-verification",false,"I verified this exact public event URL is live")
  must(f.Parse(args))
  bundle,ledger:=load(*b,*l)
  must(gigops.RecordLive(&ledger,bundle,*source,*provider,*listing,*confirmed,time.Now()))
  must(gigops.SaveLedger(ledgerPath(*l),ledger))
  fmt.Println("Recorded previously verified public URL locally. No external site action performed.")
}
