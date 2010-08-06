// KRPC
// - query
// - response
// - error
//
// RPCs:
//      ping:
//         see if node is reachable and save it on routing table.
//      find_node:
//	   run when DHT node count drops, or every X minutes. Just to
//   	   ensure our DHT routing table is still useful.
//      get_peers:
//	   the real deal. Iteratively queries DHT nodes and find new
//         sources for a particular infohash.
//	announce_peer:
//         announce that this node is downloading a torrent.
//
// Reference:
//     http://www.bittorrent.org/beps/bep_0005.html
//
// Keep in mind: handle and route each incoming connection using only
// ephemerous data. Don't keep too much state in memory.

package taipei

import (
	"bytes"
	"fmt"
	"jackpal/bencode"
	"log"
	"net"
	"os"
)

// Owned by the DHT engine.
type DhtRemoteNode struct {
	address           string
	lastTransactionID int // should be incremented after consumed.
	peerID            string
	localNode         *DhtEngine
}

const (
	// Very arbitrary value.
	PING_RES_LEN = 100
)

// Called by the DHT engine. Should return immediately.
func newRemoteNode(n *DhtEngine, address string) (r *DhtRemoteNode) {
	r = &DhtRemoteNode{
		address:           address,
		lastTransactionID: 2, // Initial value.
		peerID:            "",
		localNode:         n,
	}
	// Find if node is reachable.
	go r.handshake()
	return

}
// Ping node. If it replies, notify DHT engine. Otherwise just quit and this
// DhtRemoteNode is never used anymore and will be garbage collected, hopefully.
// We will lose the transaction ID, but who cares. In the future, we may want
// to keep this for a while, as a blacklist.
//
// Should run as go routine by DHT engine. Someone must read from
// r.localNode.handshakeWait() at some point otherwise this will block forever.
func (r *DhtRemoteNode) handshake() {
	t := r.newTransaction()
	p, _ := r.encodedPing(t)
	response, err := r.sendMsg(p)
	if err != nil {
		log.Stderr("Handshake error with node", r.address, err.String())
		return
	}
	rt, ok := response["t"].(string)
	if ok && rt == string(t) {
		// Good, a valid reply to our ping. Add to good hosts list.
		r.localNode.handshakeResults <- r
	} else {
		log.Stderrf("wrong transaction id %v, want %v.", rt, string(t))
	}
}


// encodedPing returns the bencoded string to be used for DHT ping queries.
func (r *DhtRemoteNode) encodedPing(transId string) (msg string, err os.Error) {
	queryArguments := map[string]string{"id": r.localNode.peerID}
	msg, err = encodeMsg("ping", queryArguments, transId)
	return
}

// encodedGetPeers returns the bencoded string to be used for DHT get_peers queries.
func (r *DhtRemoteNode) encodedGetPeers(transId string, infohash string) (msg string, err os.Error) {
	queryArguments := map[string]string{
		"id":        r.localNode.peerID,
		"info_hash": infohash,
	}
	msg, err = encodeMsg("get_peers", queryArguments, transId)
	return

}

func (r *DhtRemoteNode) newTransaction() string {
	// TODO: Find a better way to convert int to string. strconv.Itoa()
	// didnt seem to work, neither did string().
	t := fmt.Sprintf("%d", r.lastTransactionID)
	r.lastTransactionID = (r.lastTransactionID + 1) % 256
	return t
}

// Sends a message to the remote node.
// msg should be the bencoded string ready to be sent in the wire.
func (r *DhtRemoteNode) sendMsg(msg string) (response map[string]interface{}, err os.Error) {
	conChan := make(chan net.Conn)
	log.Stderrf("Sending msg %s (len=%d) to %s", msg, len(msg), r.address)
	go r.dialNode(conChan)
	c := <-conChan
	if _, err := c.Write(bytes.NewBufferString(msg).Bytes()); err != nil {
		log.Stderr("dht node write failed", err.String())
		return
	}
	// TODO: Instead of waiting for a response here, we should exit, and
	// have a separate goroutine for handling all incoming queries.
	if response, err = readResponse(c, PING_RES_LEN); err != nil {
		return
	}
	return
}

func (r *DhtRemoteNode) dialNode(ch chan net.Conn) {
	conn, err := net.Dial("udp", "", r.address)
	if err == nil {
		conn.SetReadTimeout(3 * NS_PER_S)
		ch <- conn
	}
	return
}

// Read responses from bencode-speaking nodes. Return the appropriate data structure.
func readResponse(c net.Conn, length int) (response map[string]interface{}, err os.Error) {
	// The calls to bencode.Unmarshal() can be fragile.
	defer func() {
		if x := recover(); x != nil {
			log.Stderrf("!!! Recovering from panic() after bencode.Unmarshal")
		}
	}()
	buf := make([]byte, length)
	log.Stderrf("Reading...")
	if _, err = c.Read(buf); err != nil {
		return
	} else {
		log.Stderrf("====> response received %v (len=%s)", string(buf), len(buf))
	}
	// I can't make the bencode package fill in the inner dictionary inside
	// "d", so I can't get the peer ID of the node that replied. Annoying,
	// but I don't currently need it.
	response = map[string]interface{}{}
	err = bencode.Unmarshal(bytes.NewBuffer(buf), &response)
	// log.Stderrf("%+v", reply)
	return
}
func encodeMsg(queryType string, queryArguments map[string]string, transId string) (msg string, err os.Error) {
	query := map[string]interface{}{
		"t": transId,
		"y": "q",
		"q": queryType,
		"a": queryArguments,
	}
	var b bytes.Buffer
	if err = bencode.Marshal(&b, query); err != nil {
		log.Stderr("bencode error: " + err.String())
		return
	}
	msg = string(b.Bytes())
	return
}
