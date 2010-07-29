package main

import (
	"flag"
	"log"
	"taipei"
)

var torrent *string = flag.String("torrent", "", "URL or path to a torrent file")
var debugp *bool = flag.Bool("debug", false, "Turn on debugging")


func main() {
	// testBencode()
	// testUPnP()
	// Taipei
	taipei.FileDir = flag.String("fileDir", ".", "path to directory where files are stored")
	taipei.Port = flag.Int("port", 0, "Port to listen on. Defaults to random.")
	taipei.UseUPnP = flag.Bool("useUPnP", false, "Use UPnP to open port in firewall.")
	flag.Parse()
	log.Stderr("Starting.")
	dht := &taipei.DhtNode{"abcdefghij0123456789"}
	r, _ := dht.Ping()
	log.Stderr("DHT: ", r)
	ts, err := taipei.NewTorrentSession(*torrent)
	if err != nil {
		log.Stderr("Could not create torrent session.", err)
		return
	}
	err = ts.DoTorrent()
	if err != nil {
		log.Stderr("Failed: ", err)
	} else {
		log.Stderr("Done")
	}
}
