package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/dustin/go-humanize"
	"golang.org/x/sys/unix"
)

var mindsk uint64
var techulusPushAPIKey string
var name string
var lastpush time.Time

func main() {

	sizestr := os.Getenv("MIN_DISK_SPACE")
	if sizestr == "" {
		sizestr = "1GB"
	}
	techulusPushAPIKey = os.Getenv("TECHULUS_PUSH_API_KEY")

	name = os.Getenv("DISKALERT_NAME")
	if name == "" {
		name = "diskalert node"
	}

	size, err := humanize.ParseBytes(sizestr)
	if err != nil {
		println("invalid MIN_DISK_SPACE format")
		os.Exit(1)
	}
	mindsk = size

	closech := make(chan struct{})

	sigch := make(chan os.Signal, 1)

	signal.Notify(sigch, os.Interrupt)

	go runloop(closech)

	<-sigch
	close(closech)
}

func runloop(ch <-chan struct{}) {
	for {
		select {
		case <-ch:
			return
		case <-time.After(time.Second):
			dskalert()
		}
		select {
		case <-ch:
			return
		case <-time.After(time.Second * 119):
			// continue
		}

	}
}

func dskalert() {
	var stat unix.Statfs_t
	wd, err := os.Getwd()
	if err != nil {
		println("failed to get wd")
	}
	unix.Statfs(wd, &stat)
	// Available blocks * size per block = available space in bytes
	avbytes := stat.Bavail * uint64(stat.Bsize)
	println(humanize.Bytes(avbytes) + " available")
	if avbytes < mindsk {
		println("alert!!!")
		fmt.Printf("size %s < limit (%s)\n", humanize.Bytes(avbytes), humanize.Bytes(mindsk))
		if lastpush.Add(time.Hour * 4).Before(time.Now()) {
			title := url.QueryEscape("diskalert - " + name)
			body := url.QueryEscape(fmt.Sprintf("size %s < limit (%s)", humanize.Bytes(avbytes), humanize.Bytes(mindsk)))
			if resp, err := http.Get(fmt.Sprintf("https://push.techulus.com/api/v1/notify/%s?title=%s&body=%s", techulusPushAPIKey, title, body)); err == nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				lastpush = time.Now()
			}
		}
	}
}
