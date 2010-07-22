package main

import (
	"fmt"
	"http"
	"log"
)

// GlobalStatus provides handlers for the web server, as well as channels for
// communicating with the torrent engine.
type GlobalStatus struct {
	webSessionInfo chan SessionInfo // TorrentSession.DoTorrent() will create the channel and populate it.
	// Other channels will go here.
}

func (ss *GlobalStatus) ServeHTTP(c *http.Conn, req *http.Request) {
	if ss.webSessionInfo != nil {
		log.Stderr("Trying to fetch SessionInfo")
		ssi := <-ss.webSessionInfo
		// TODO: Use a template.
		fmt.Fprintf(c, ssi.String())
	}
}
