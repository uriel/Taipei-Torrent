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
}

const (
	// Very arbitrary value.
	MAX_RES_LEN      = 1000000
	NODE_CONTACT_LEN = 26
	PEER_CONTACT_LEN = 6
)

// Called by DHT server or torrent server. Should return immediately.
func (d *DhtEngine) newRemoteNode(id string, address string) (r *DhtRemoteNode) {
	r = &DhtRemoteNode{
		address:           address,
		lastTransactionID: rand.Intn(255) + 1,  // Doesn't have to be crypto safe.
		id:                id,
		localNode:         d,
	}
	return

}
// Ping node. If it replies, notify DHT engine. Otherwise just quit and this
// DhtRemoteNode is never used anymore and will be garbage collected, hopefully.
// We will lose the transaction ID, but who cares. In the future, we may want
// to keep this for a while, as a blacklist.
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
		// Good, a valid reply to our ping. Add to good hosts list.
		r.localNode.handshakeResults <- r
	} else {
		// TODO: should try again because they may have responded to a previous query from us.
		// As it is, only one transaction per remote node may be active, which of course is too restrict.
		log.Stderrf("wrong transaction id %v, want %v.", response.T, string(t))
	}
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
		log.Stderr("GetPeers query to host failed", r.address, err.String())
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
		r := r.localNode.newRemoteNode(id, address)
		if values := r.recursiveGetPeers(infoHash, count-1); values != nil {
			return values
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
	if response, err = readResponse(c, MAX_RES_LEN); err != nil {
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
func readResponse(c net.Conn, length int) (response responseType, err os.Error) {
	// The calls to bencode.Unmarshal() can be fragile.
	defer func() {
		if x := recover(); x != nil {
			log.Stderrf("!!! Recovering from panic() after bencode.Unmarshal")
		}
	}()
	buf := make([]byte, length)
	if _, err = c.Read(buf); err != nil {
		return
	} else {
		buf = bytes.Trim(buf, string(0))
	}
	err = bencode.Unmarshal(bytes.NewBuffer(buf), &response)
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
