package taipei

import (
	"jackpal/bencode"
	"bytes"
	//"net"
	"os"
	"log"
)

type DhtNode struct {
	PeerID string
	//	transactionID int // incremented every time.
}

func (n *DhtNode) Ping() (r string, err os.Error) {
	var queryA = map[string]string{
		"id": n.PeerID,
	}

	var a bytes.Buffer
	bencode.Marshal(&a, queryA)

	var pingMessage = map[string]interface{}{
		"t": "aa", // "random" transaction id.
		"y": "q",
		"q": "ping",
		"a": queryA,
	}

	var b bytes.Buffer
	if err = bencode.Marshal(&b, pingMessage); err != nil {
		log.Stderr("bencode error" + err.String())
		return
	}
	r = string(b.Bytes())
	log.Stderr("bencode result", r)
	if r != "d1:ad2:id20:abcdefghij0123456789e1:q4:ping1:t2:aa1:y1:qe" {
		log.Stderr("wrong string")
	}
	return
	//conn, err := net.Dial("udp", "", "router.bittorrent.com:6881")
	//	if _, err = conn.Write(b.Bytes()); err != nil {
	//		log.Stderr("conn error" + err.String())
	//		return
	//	}
	//	var r []byte
	//	if _, err = conn.Read(r); err != nil {
	//		log.Stderr("read error" + err.String())
	//	}
	//	log.Stderr("peer result", string(r))
}
