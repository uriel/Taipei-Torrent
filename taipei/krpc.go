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
	"jackpal/bencode"
	"log"
	"net"
	"os"
	"rand"
	"strconv"
	"time"
)

func init() {
	rand.Seed(int64(time.Nanoseconds() % (1e9 - 1)))
}

// Owned by the DHT engine.
type DhtRemoteNode struct {
	address string
	id      string
	// lastTransactionID should be incremented after consumed. Based on the
	// protocol, it would be two letters, but I'm using 0-255, although
	// treated as string.
	lastTransactionID int
	localNode         *DhtEngine
	reachable	bool
}

const (
	NODE_CONTACT_LEN = 26
	PEER_CONTACT_LEN = 6
	UDP_READ_TIMEOUT = 1e9	// one second.
	UDP_READ_RETRY = 3
)

// Called by DHT server or torrent server. Should return immediately.
func (d *DhtEngine) newRemoteNode(id string, address string) (r *DhtRemoteNode) {
	r = &DhtRemoteNode{
		address:           address,
		lastTransactionID: rand.Intn(255) + 1,  // Doesn't have to be crypto safe.
		id:                id,
		localNode:         d,
		reachable:	   false,
	}
	return

}
// Ping node. Notify DHT engine via r.localNode.handshakeResults wether it
// replies or not.
//
// Should run as go routine by DHT engine. Caller must read from
// r.localNode.handshakeWait() at some point otherwise this will block forever.
func (r *DhtRemoteNode) handshake() {
	t := r.newTransaction()
	p, _ := r.encodedPing(t)
	response, err := r.sendMsg(p)
	// TODO: Move these error checkings to sendMsg. Maybe make a common object.
	if err != nil {
		log.Stderr("Handshake error with node", r.address, err.String())
		return
	}
	if response.T == string(t) {
		r.reachable = true
		// Good, a valid reply to our ping, mark it as reachable.
	} else {
		// TODO: should try again because they may have responded to a previous query from us.
		// As it is, only one transaction per remote node may be active, which of course is too restrict.
		log.Stderrf("wrong transaction id %v, want %v.", response.T, string(t))
	}
	r.localNode.handshakeResults <- r
}

// Contacts this node asking for closest sources for the specified infohash,
// recursive, decreasing count each time until it reaches zero.  returns
// map[string]int, where key are addresses and value is an int that can be
// ignored.
func (r *DhtRemoteNode) recursiveGetPeers(infoHash string, count int) (peers map[string]int) {
	t := r.newTransaction()
	m, _ := r.encodedGetPeers(t, infoHash)
	response, err := r.sendMsg(m)
	if err != nil {
		return
	}
	if response.T != string(t) {
		log.Stderrf("wrong transaction id %v, want %v.", response.T, string(t))
		return
	}
	// Mark node as reachable.
	r.localNode.handshakeResults <- r

	values := response.R.Values
	if values != nil {
		// FANTASTIC!!
		// TODO: can also be a list..
		log.Stdoutf("GetPeers l=%d ======>>>> FANTASTIC, got VALUES! Thanks %s", count, r.address)
		p := map[string]int{}
		i := 0
		for _, n := range values {
			if len(n) != PEER_CONTACT_LEN {
				// TODO: Err
				log.Stderrf("Invalid length of node contact info.")
				log.Stderrf("Should be == %d, got %d", PEER_CONTACT_LEN, len(n))
				break
			}
			address := binaryToDottedPort(n)
			// TODO: program locks if address contains an invalid hostname..
			p[address] = 0
			i++
		}
		log.Stdoutf("----->>> %+v", p)
		return p
	}
	// Oh noes, got nodes instead. We'll need to recurse.
	if count == 0 {
		return nil
	}
	log.Stdoutf("GetPeers l=%d => Didn't get peers, but got closer nodes (len=%d)", count, len(response.R.Nodes))
	nodes := response.R.Nodes
	if nodes == "" {
		return nil
	}
	// TODO: Check if the "distance" for nodes provided are lower than what we already have.
	for id, address := range parseNodesString(nodes) {
		// Skip nodes we know already.
		if _, ok := r.localNode.nodes[id]; !ok {
			r := r.localNode.newRemoteNode(id, address)
			if values := r.recursiveGetPeers(infoHash, count-1); values != nil {
				return values
			}
		}
	}
	// TODO: update routing table.
	// TODO: announce_peers to peers.
	return
}

// The 'nodes' response is a string with fixed length contacts concatenated arbitrarily.
func parseNodesString(nodes string) (parsed map[string]string) {
	//log.Stdoutf("nodesString: %x", nodes)
	parsed = make(map[string]string)
	if len(nodes)%NODE_CONTACT_LEN > 0 {
		// TODO: Err
		log.Stderrf("Invalid length of nodes.")
		log.Stderrf("Should be a multiple of %d, got %d", NODE_CONTACT_LEN, len(nodes))
		return
	}
	// make this a struct instead because we also need to provide the infohash.
	// TODO: I dont know why I said we need to provide the infohash.. hehe.
	for i := 0; i < len(nodes); i += NODE_CONTACT_LEN {
		id := nodes[i : i+19]
		address := binaryToDottedPort(nodes[i+20 : i+26])
		parsed[id] = address
	}
	//log.Stdoutf("parsed: %+v", parsed)
	return

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
	t := strconv.Itoa(r.lastTransactionID)
	r.lastTransactionID = (r.lastTransactionID + 1) % 256
	return t
}

type getPeersResponse struct {
	// TODO: argh, values can be a string depending on the client (e.g: original bittorrent).
	Values []string "values"
	Id     string   "id"
	Nodes  string   "nodes"
}

type responseType struct {
	T string           "t"
	Y string           "y"
	Q string           "q"
	R getPeersResponse "R"
}

// Sends a message to the remote node.
// msg should be the bencoded string ready to be sent in the wire.
func (r *DhtRemoteNode) sendMsg(msg string) (response responseType, err os.Error) {
	conChan := make(chan net.Conn)
	//log.Stdoutf("Sending msg %q (len=%d) to %s", msg, len(msg), r.address)
	go r.dialNode(conChan)
	c := <-conChan
	if _, err := c.Write(bytes.NewBufferString(msg).Bytes()); err != nil {
		log.Stderr("dht node write failed", err.String())
		return
	}
	// TODO: This is broken. Responses can be delayed and come out of
	// order, and we don't want to block here forever waiting for a
	// response. Instead we should exit, and have a separate goroutine for
	// handling all incoming queries.
	if response, err = readResponse(c); err != nil {
		return
	}
	return
}

func (r *DhtRemoteNode) dialNode(ch chan net.Conn) {
	conn, err := net.Dial("udp", "", r.address)
	if err == nil {
		// The more we wait the better, because these nodes can be very
		// slow.  But that means whatever goroutine reads from this
		// connection will block for a long time, so keep that in mind.
		conn.SetReadTimeout(UDP_READ_TIMEOUT)
		ch <- conn
	}
	return
}

// Read responses from bencode-speaking nodes. Return the appropriate data structure.
// TODO: should listen to all data coming into the port, then deciding what to
// do, based on peer ID and transaction ID.
func readResponse(c net.Conn) (response responseType, err os.Error) {
	// The calls to bencode.Unmarshal() can be fragile.
	defer func() {
		if x := recover(); x != nil {
			log.Stderrf("!!! Recovering from panic() after bencode.Unmarshal")
		}
	}()

	// Currently, tries to read from socket up to 3 times, blocking each
	// read for one second (or UDP_READ_TIMEOUT), waiting for all data to
	// come.  If partial data can be bdecoded, stop reading from socket and
	// return it.
	// I'm sure there are better ways. For example, could instead read X
	// bytes each time.
	var Buf bytes.Buffer
	i := 0
	for i < UDP_READ_RETRY {
		i++
		var n int64
		n, err = Buf.ReadFrom(c)
		if err == nil {
			// Should never happen, always returns os.EAGAIN at least.
			log.Stderr("readResponse: got err == nil, which is not expected. This is a bug.")
			continue
		}
		if n == 0 {
			continue
		}
                if e, ok := err.(*net.OpError); ok && e.Error == os.EAGAIN {
			// Since this is UDP, it should be quite common that we
			// get partial data. Just discard it if that's the
			// case.
			if e2 := bencode.Unmarshal(bytes.NewBuffer(Buf.Bytes()), &response); e2 == nil {
				err = nil
				//log.Stderrf("client=%q, GOOD! unmarshal finished. d=(%q)", c.RemoteAddr(), Buf.String())
				break
			} else {
				log.Stdoutf("DEBUG client=%q, unmarshal error, odd or partial data during UDP read? d=(%q), err=%s", c.RemoteAddr(), Buf.String(), e2.String())
			}
		} else {
			log.Stderrf("readResponse client=%s, Unexpected error: %s", c.RemoteAddr(), e.Error.String())
		}
	}
	return
}

func encodeMsg(queryType string, queryArguments map[string]string, transId string) (msg string, err os.Error) {
	type structNested struct {
		T string            "t"
		Y string            "y"
		Q string            "q"
		A map[string]string "a"
	}
	query := structNested{transId, "q", queryType, queryArguments}
	var b bytes.Buffer
	if err = bencode.Marshal(&b, query); err != nil {
		log.Stderr("bencode error: " + err.String())
		return
	}
	msg = string(b.Bytes())
	return
}
