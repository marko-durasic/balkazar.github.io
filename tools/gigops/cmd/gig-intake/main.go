package main

// CLI bridge for AI-extracted structured GigIntake JSON, not a natural
// language model. By default read-only; --store changes private local state
// only. No third-party post, event publication, message or payment is possible.
import (
 "encoding/json"
 "flag"
 "fmt"
 "io"
 "os"
 "time"
 gigops "balkazar.org/musicops"
)
func run()error{
 file:=flag.String("file","","Input JSON; omit to read from stdin")
 store:=flag.String("store","","Optional private absolute .json path for durable campaign reconciliation")
 flag.Parse()
 var r io.Reader=os.Stdin
 if *file!=""{
   f,err:=os.Open(*file);if err!=nil{return err};defer f.Close();r=f
 }
 var g gigops.GigIntake
 decoder:=json.NewDecoder(io.LimitReader(r,1<<20))
 decoder.DisallowUnknownFields()
 if err:=decoder.Decode(&g);err!=nil{return fmt.Errorf("gig JSON: %w",err)}
 var extra any
 if err:=decoder.Decode(&extra);err!=io.EOF{return fmt.Errorf("extra JSON after gig: %v",err)}
 enc:=json.NewEncoder(os.Stdout);enc.SetIndent("","  ")
 if *store!="" {
    result,err:=gigops.ApplyGigIntakeFile(*store,g,time.Now())
    if err!=nil{return err}
    return enc.Encode(result)
 }
 plan,err:=gigops.PlanGigIntake(g,time.Now())
 if err!=nil{return err}
 return enc.Encode(plan)
}
func main(){if err:=run();err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(2)}}
