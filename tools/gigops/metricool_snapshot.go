package gigops

// Metricool connector snapshots are read-only scheduling observations.
// An observed draft or queued post is NOT a public post, publication proof,
// endorsement of its contents, or permission to schedule future posts.
import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "regexp"
    "sort"
    "strings"
    "time"
)

const balkazarMetricoolBrandID = "7273414"
var metricoolPostIDRE = regexp.MustCompile("^[0-9]{1,24}$")
var metricoolUUIDRE = regexp.MustCompile("^[A-Za-z0-9_-]{4,80}$")

type metricoolProvider struct {
    Network string `json:"network"`
    Status string `json:"status"`
}
type metricoolPost struct {
    ID json.Number `json:"id"`
    UUID string `json:"uuid"`
    Text string `json:"text"`
    Draft bool `json:"draft"`
    AutoPublish bool `json:"autoPublish"`
    PublicationDate struct {
        DateTime string `json:"dateTime"`
        Timezone string `json:"timezone"`
    } `json:"publicationDate"`
    Providers []metricoolProvider `json:"providers"`
    Media []string `json:"media"`
}
type metricoolPayload struct {
    Data []metricoolPost `json:"data"`
}

// MetricoolPostObservation deliberately contains no captions, media URLs,
// credentials or personal data. A snapshot is caller-supplied and may be stale
// or forged; 'observed' never means independent platform verification.
type MetricoolPostObservation struct {
    PostUUID string `json:"post_uuid"`
    PlannerPostID string `json:"planner_post_id"`
    State string `json:"state"` // observed-draft, observed-queued, needs-review
    AutoPublish bool `json:"auto_publish"`
    PublicationAt string `json:"publication_at"`
    Networks []string `json:"networks"`
    ContentSHA256 string `json:"content_sha256"`
}
type MetricoolReadbackPreview struct {
    ReadOnly bool `json:"read_only"`
    PublicWrites int `json:"public_writes"`
    Source string `json:"source"` // supplied-connector-snapshot, not attested proof
    BrandID string `json:"brand_id"`
    GigID string `json:"gig_id"`
    MatchedPosts []MetricoolPostObservation `json:"matched_posts"`
    OtherPostCount int `json:"other_post_count"`
    Warnings []string `json:"warnings"`
}

func exactEventLinkInText(text, canonical string) bool {
    // Prevent treating a lookalike domain or a longer unrelated URL as a match.
    for _, token := range strings.Fields(text) {
        token = strings.Trim(token, " \t\n\r<>[](){}\"'.,;!?")
        if token == canonical { return true }
    }
    return false
}

func PreviewMetricoolSnapshot(reader io.Reader, gig GigIntake, brandID string) (MetricoolReadbackPreview,error) {
    if brandID != balkazarMetricoolBrandID || gig.Artist!="Balkazar" {
        return MetricoolReadbackPreview{},errors.New("Metricool brand and artist do not match approved Balkazar tenant")
    }
    if !ValidSourceURL(gig.OfficialSourceURL) || gig.GigID=="" {
        return MetricoolReadbackPreview{},errors.New("gig must have a stable id and canonical Balkazar event URL")
    }
    dec:=json.NewDecoder(io.LimitReader(reader,4<<20))
    dec.UseNumber()
    var payload metricoolPayload
    if err:=dec.Decode(&payload);err!=nil{return MetricoolReadbackPreview{},fmt.Errorf("invalid Metricool snapshot: %w",err)}
    var extra any
    if err:=dec.Decode(&extra);err!=io.EOF{
        return MetricoolReadbackPreview{},errors.New("Metricool snapshot has trailing data or exceeds 4 MiB")
    }
    if payload.Data==nil{return MetricoolReadbackPreview{},errors.New("Metricool snapshot must include data array")}
    if len(payload.Data)>500{return MetricoolReadbackPreview{},errors.New("Metricool snapshot exceeds 500 posts")}
    p:=MetricoolReadbackPreview{
        ReadOnly:true,PublicWrites:0,Source:"supplied-connector-snapshot",
        BrandID:brandID,GigID:gig.GigID,
        MatchedPosts:[]MetricoolPostObservation{},Warnings:[]string{},
    }
    seen:=map[string]bool{}
    hashToUUID:=map[string]string{}
    for _,post:=range payload.Data {
        id:=post.ID.String()
        if !metricoolPostIDRE.MatchString(id)||!metricoolUUIDRE.MatchString(post.UUID){
            return MetricoolReadbackPreview{},errors.New("snapshot contains invalid provider post identifier")
        }
        if seen[post.UUID] {return MetricoolReadbackPreview{},fmt.Errorf("duplicate post UUID in snapshot: %s",post.UUID)}
        seen[post.UUID]=true
        if !exactEventLinkInText(post.Text,gig.OfficialSourceURL) {
            p.OtherPostCount++
            continue
        }
        if len(post.Text)>20000 {return MetricoolReadbackPreview{},errors.New("matched post text exceeds bound")}
        if len(post.Providers)==0 || len(post.Providers)>10 {return MetricoolReadbackPreview{},errors.New("matched post has invalid network list")}
        loc,err:=time.LoadLocation(post.PublicationDate.Timezone)
        if err!=nil || post.PublicationDate.Timezone!=gig.Timezone {
            return MetricoolReadbackPreview{},errors.New("matched post timezone disagrees with gig")
        }
        publication,err:=time.ParseInLocation("2006-01-02T15:04:05",post.PublicationDate.DateTime,loc)
        if err!=nil{return MetricoolReadbackPreview{},errors.New("invalid scheduled publication timestamp")}
        networks:=make([]string,0,len(post.Providers))
        seenNetworks:=map[string]bool{}
        allPending:=true
        for _,prov:=range post.Providers {
            if !allowed(prov.Network,"facebook","instagram","threads","youtube","tiktok","bluesky","twitter","linkedin","pinterest","gmb") ||
                seenNetworks[prov.Network] {return MetricoolReadbackPreview{},errors.New("unsupported or repeated Metricool network")}
            seenNetworks[prov.Network]=true
            networks=append(networks,prov.Network)
            if prov.Status!="PENDING"{allPending=false}
        }
        sort.Strings(networks)
        state:="needs-review"
        if post.Draft {
            state="observed-draft"
        } else if allPending {
            state="observed-queued"
        }
        // Hash creative content but do not persist private captions or media.
        bytes,_:=json.Marshal([]any{post.Text,post.Media,networks})
        sum:=sha256.Sum256(bytes)
        hash:=hex.EncodeToString(sum[:])
        if previous,ok:=hashToUUID[hash];ok && previous!=post.UUID{
            p.Warnings=append(p.Warnings,"Possible duplicate creative across two planner posts; inspect before any new scheduling")
        }
        hashToUUID[hash]=post.UUID
        p.MatchedPosts=append(p.MatchedPosts,MetricoolPostObservation{
            PostUUID:post.UUID,PlannerPostID:id,State:state,
            AutoPublish:post.AutoPublish,PublicationAt:publication.Format(time.RFC3339),
            Networks:networks,ContentSHA256:hash,
        })
        if state=="needs-review" {
            p.Warnings=append(p.Warnings,"Provider reports a non-draft post outside the all-PENDING state; inspect separately, do not infer it was published")
        }
    }
    sort.Slice(p.MatchedPosts,func(i,j int)bool{return p.MatchedPosts[i].PostUUID<p.MatchedPosts[j].PostUUID})
    return p,nil
}
