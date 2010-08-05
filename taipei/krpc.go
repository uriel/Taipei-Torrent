package taipei

import (
	"jackpal/bencode"
	"bytes"
	"fmt"
	"net"
	"os"
	"log"
)

// Owned by the DHT engine.
// TODO: this is pointed to from within the DhtNode, which is owned by the torrent
// engine. Any race conditions?
type DhtRemoteNode struct {
	address           string
	lastTransactionID int // should be incremented after consumed.
	peerID            string
	localNode         *DhtEngine
	// TODO: the consumer of handshake results will be the DHT engine, but we
	// cant expect it to read from each of the remote node channels.
	// We should instead update a DhtNode channel with a tuple of 'node foo'
	// is UP/DOWN. So get rid of this.
	// handshakeWait chan bool
}

// KRPC
// - query
// - response
// - error
// Possible design decision: do not keep connection state for each peer. Handle and
// route each incoming connection using the same data, as done in torrent engine.

const (
	PING_REQ_LEN = 56
	// Very arbitrary value.
	PING_RES_LEN = 100
)

// We've got a new node id. We need to:
// - ping it and see if it's reachable. Ignore otherwise.
// - save it on our list of good nodes.
// - later, we'll implement bucketing, etc.
//
// Called by the torrent engine from the main goroutine, and should return immediately.
func NewRemoteNode(n *DhtEngine, address string) (r *DhtRemoteNode, err os.Error) {
	r = &DhtRemoteNode{
		address:           address,
		lastTransactionID: 2, // Initial value.
		peerID:            "",
		localNode:         n,
	}
	// Find if node is reachable.
	go r.Handshake()
	// No current way to detect err.
	return

}

// Ping node and see if it replies. Then it updates the nodes list to mark
// reachable ones, although later this will be done by the main torrent engine.
//
// Should run as go routine by DHT engine.
// Someone must read from r.localNode.handshakeWait() at some point otherwise this will block.
func (r *DhtRemoteNode) Handshake() {
	conChan := make(chan net.Conn)
	go dialNode(r.address, conChan)
	c := <-conChan
	// TODO: Find a better way to convert int to string. strconv.Itoa()
	// didnt seem to work, neither did string().
	t := fmt.Sprintf("%d", r.lastTransactionID)
	r.lastTransactionID = (r.lastTransactionID + 1) % 256
	p, _ := r.ping(t)

	log.Stderrf("Sending ping %s (len=%d) to %s", p, len(p), r.address)
	if _, err := c.Write(bytes.NewBufferString(p).Bytes()); err != nil {
		log.Stderr("dht node write failed", err.String())
	} else {
		log.Stderr("sent dht ping successfully to node", r.address)
	}
	if resp, err := readResp(c, PING_RES_LEN); err != nil {
		log.Stderrf("Handshaking failed %v", err.String())
	} else {
		rt := resp["t"].(string)
		if rt == string(t) {
			// Good, a reply to our ping. Add to good hosts list.
			log.Stderrf("Node %v => good node", r.address)
			r.localNode.handshakeResults <- r
		} else {
			log.Stderrf("wrong transaction id %v, want %v.", rt, string(t))
		}
	}
	// Ugly. Signals a failure for the unit tests.
	r.localNode.handshakeResults <- nil
	return
}


// Ping returns the bencoded string to be used for DHT ping queries.
func (r *DhtRemoteNode) ping(transId string) (msg string, err os.Error) {
	queryArguments := map[string]string{"id": r.localNode.PeerID}
	pingMessage := map[string]interface{}{
		"t": transId,
		"y": "q",
		"q": "ping",
		"a": queryArguments,
	}
	var b bytes.Buffer
	if err = bencode.Marshal(&b, pingMessage); err != nil {
		log.Stderr("bencode error: " + err.String())
		return
	}
	msg = string(b.Bytes())
	return
}

// Read responses from bencode-speaking nodes. Return the appropriate data structure.
func readResp(c net.Conn, length int) (resp map[string]interface{}, err os.Error) {
	// The calls to bencode.Unmarshal() can be fragile.
	defer func() {
		if x := recover(); x != nil {
			log.Stderrf("!!! Recovering from panic() after bencode.Unmarshal")
		}
	}()
	buf := make([]byte, length)
	log.Stderrf("Reading...")
	if _, err = c.Read(buf); err != nil {
		log.Stderrf("Read failure!!! %v", err.String())
		return
	} else {
		log.Stderrf("====> response received %v (len=%s)", string(buf), len(buf))
	}
	// I can't make the bencode package fill in the inner dictionary inside "d".
	resp = map[string]interface{}{}
	err = bencode.Unmarshal(bytes.NewBuffer(buf), &resp)
	log.Stderrf("===>ID: %+v", resp)
	return
}

func dialNode(node string, ch chan net.Conn) {
	conn, err := net.Dial("udp", "", node)
	if err == nil {
		ch <- conn
	}
	return
}

//     Other RPCs:
//     find_node: run when DHT node count drops, or every X minutes. Just to
//                ensure our DHT routing table is still useful.
//     get_peers: the real deal. Iteratively queries DHT nodes and find new
//                sources for a particular infohash.
