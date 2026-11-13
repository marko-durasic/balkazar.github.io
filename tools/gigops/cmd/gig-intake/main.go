package main

// Read-only CLI bridge for AI-extracted GigIntake JSON. Does not parse natural
// language itself and never writes to websites, socials, accounts or cloud.
import (
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "os"
    "time"

    gigops "balkazar.org/musicops"
)

func run() error {
    file:=flag.String("file","","Input JSON file path; omitted reads stdin")
    flag.Parse()
    var r io.Reader=os.Stdin
    if *file!="" {
        f,err:=os.Open(*file);if err!=nil{return err};defer f.Close();r=f
    }
    var g gigops.GigIntake
    decoder:=json.NewDecoder(io.LimitReader(r,1<<20))
    decoder.DisallowUnknownFields()
    if err:=decoder.Decode(&g);err!=nil{return fmt.Errorf("gig JSON: %w",err)}
    var excess any
    if err:=decoder.Decode(&excess);err!=io.EOF{return fmt.Errorf("extra JSON after gig: %v",err)}
    p,err:=gigops.PlanGigIntake(g,time.Now());if err!=nil{return err}
    enc:=json.NewEncoder(os.Stdout);enc.SetIndent("","  ")
    return enc.Encode(p)
}
func main(){if err:=run();err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(2)}}
