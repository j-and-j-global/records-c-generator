package main

import (
    "encoding/csv"
    "flag"
    "fmt"
    "io"
    "os"
    "strings"
    "time"

    "github.com/cenkalti/backoff"
    mb "github.com/michiwend/gomusicbrainz"
)

var (
    inputFile = flag.String("i", "records.csv", "CSV file containing records to generate")

    musicbrainz *mb.WS2Client
)

type Record struct {
    Title  string `json:"title"`
    Artist string `json:"artist"`
    Case   string
    Pos    int
    Tracks []Track `json:"tracks"`

    musicBrainz string `json:"-"`
}

type Track struct {
    Track string `json:"track"`
    Title string `json:"title"`
}

func main() {
    flag.Parse()

    f, err := os.Open(*inputFile)
    if err != nil {
        panic(err)
    }

    r := csv.NewReader(f)
    records := make([]Record, 0)

    headers, err := r.Read()
    if err != nil {
        panic(err)
    }

    musicbrainz, err = mb.NewWS2Client(
        "https://musicbrainz.org/ws/2",
        "Enrich my record collection",
        "1.0.0",
        "james at zero-internet dot org dot uk")
    if err != nil {
        panic(err)
    }

    idx := 0
    for {
        idx++

        line, err := r.Read()
        if err == io.EOF {
            break
        }

        if err != nil {
            panic(err)
        }

        m := lineToMap(headers, line)

        fmt.Fprintf(os.Stderr, "                                                                                                                                                \r")
        fmt.Fprintf(os.Stderr, "%d: %q - %q (musicbrainz: %s)\r", idx, m["artist"], m["title"], m["musicbrainz"])

        record := Record{
            Title:       m["title"],
            Artist:      m["artist"],
            Tracks:      make([]Track, 0),
            Case:        m["flight case"],
            Pos:         idx,
            musicBrainz: m["musicbrainz"],
        }

        err = brainz(&record)
        if err != nil {
            panic(err)
        }

        records = append(records, record)
    }

    fmt.Fprintln(os.Stderr, "")

    t := Templater{
        Year:  time.Now().Year(),
        Items: records,
    }

    output, err := t.Template()
    if err != nil {
        panic(err)
    }

    fmt.Println(output)
}

func lineToMap(headers []string, line []string) (m map[string]string) {
    m = make(map[string]string)

    for idx := range headers {
        m[headers[idx]] = line[idx]
    }

    return
}

func brainz(r *Record) (err error) {
    if r.musicBrainz == "" {
        return
    }

    var release *mb.Release
    backoff.Retry(func() (err error) {
        release, err = musicbrainz.LookupRelease(mb.MBID(r.musicBrainz), "recordings", "artists")

        return
    }, backoff.NewExponentialBackOff())

    if len(release.Mediums) == 0 {
        return
    }

    media := release.Mediums[0]

    r.Artist = artistString(release.ArtistCredit)
    r.Title = release.Title

    for _, t := range media.Tracks {
        r.Tracks = append(r.Tracks, Track{
            Track: t.Number,
            Title: t.Recording.Title,
        })
    }

    return
}

func artistString(ac mb.ArtistCredit) (s string) {
    names := make([]string, len(ac.NameCredits))

    for idx, nc := range ac.NameCredits {
        names[idx] = nc.Artist.Name
    }

    return strings.Join(names, ",")
}
