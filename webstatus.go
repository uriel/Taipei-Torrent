package main

import (
	"fmt"
	"http"
	"log"
	"template"
)

type stats struct {
	ssi *SessionInfo // Local ephemerous copy.Owned by ServeHTTP.
	m   *MetaInfo    // Owned by ServeHTTP
}

type webTorrentStats struct {
	g   *GlobalStatusSync // Contains channels written to by the torrent engine.
}

func (w *webTorrentStats) ServeHTTP(c *http.Conn, req *http.Request) {
	s := &stats{nil, nil}
	if w.g.webMetaInfo == nil || w.g.webSessionInfo == nil {
		fmt.Fprint(c, "Stats not yet available.")
		return
	} else {
		log.Stderr("Trying to fetch SessionInfo from torrent engine.")
		ssi := <-w.g.webSessionInfo
		s.ssi = &ssi

		log.Stderr("Trying to fetch MetaInfo from torrent engine.")
		m := <-w.g.webMetaInfo
		s.m = &m
		templ.Execute(s, c)
	}
}

var fmap = template.FormatterMap{
	"html": template.HTMLFormatter,
}
var templ = template.MustParse(mainPageStr, fmap)

// Keeping the templates hard-coded simplifies deployment for now.
const mainPageStr = `
<html>
<head>
<title>Taipei Torrent</title>
</head>
<body>
Downloading {.section m}
	{# For single files, the filename is in Info.Name.}
	{# Otherwise it's in in the Path of each Info.Files}
	{Info.Name|html}.<br>
	<ol>
	{.repeated section Info.Files}
		<li>{Path|html} </li>
	{.end}
	</ol>
{.end}
<br><br>
</body>
</html>
`
