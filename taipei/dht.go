// DHT node for Taipei Torrent, for tracker-less peer information exchange.
package taipei

import (
	"expvar"
	"json"
	"log"
	"os"
)

// DhtEngine should be created by NewDhtNode(). It provides DHT features to a
// torrent client, such as finding new peers for torrent downloads without
// requiring a tracker. The client can only use the public (first letter
// uppercase) channels for communicating with the DHT goroutines.
type DhtEngine struct {
	peerID           string
	nodes            map[string]*DhtRemoteNode // key == peer ID
	handshakeResults chan *DhtRemoteNode       // DHT internal channel.

	// Public channels:
	Quit                   chan bool
	RemoteNodeAcquaintance chan *DhtNodeCandidate
	PeersNeeded            chan *InfohashPeers
	PeersNeededResults     chan *InfohashPeers // Zero-length results are valid.
	// DudeWeHeardAboutANewTorrent
}

type DhtStatsType struct {
	engines	[]*DhtEngine
}

var DhtStats DhtStatsType

func NewDhtNode(nodeId string) (node *DhtEngine, err os.Error) {
	node = &DhtEngine{
		peerID:                 nodeId,
		// TODO: cleanup bad nodes from time to time.
		nodes:                  make(map[string]*DhtRemoteNode),
		handshakeResults:       make(chan *DhtRemoteNode),
		PeersNeededResults:     make(chan *InfohashPeers),
		RemoteNodeAcquaintance: make(chan *DhtNodeCandidate),
		PeersNeeded:            make(chan *InfohashPeers),
		Quit:                   make(chan bool),
	}

	// Update list of known engines.
	s := DhtStats.engines
	s = s[0:len(s)] // We assume the cap is big enough.
	s[len(s)-1] = node
	return
}

type InfohashPeers struct {
	infoHash string
	nodes    map[string]int // key=address, value=ignored.
}


// Companion of the d.PeersNeeded channel.
func (d *DhtEngine) NewNeedDhtPeers(infoHash string) (needPeers *InfohashPeers) {
	peers := map[string]int{}
	needPeers = &InfohashPeers{infoHash, peers}
	return
}

type DhtNodeCandidate struct {
	id      string
	address string
}


// DoDht is the DHT node main loop and should be run as a goroutine by the
// torrent client. To make it quit, send any value to the Quit channel of the
// DhtEngine.
func (d *DhtEngine) DoDht() {
	log.Stdout("Starting DHT node.")
	for {
		select {
		case helloNode := <-d.RemoteNodeAcquaintance:
			// We've got a new node id. We need to:
			// - see if we know it already, skip accordingly.
			// - ping it and see if it's reachable. Ignore otherwise.
			// - save it on our list of good nodes.
			// - later, we'll implement bucketing, etc.
			if _, ok := d.nodes[helloNode.id]; !ok {
				r := d.newRemoteNode(helloNode.id, helloNode.address)
				go r.handshake()
			}
		case needPeers := <-d.PeersNeeded:
			// torrent server is asking for more peers for a particular infoHash.
			// Ask the closest nodes for directions. There is a
			// good chance that results will be empty.
			log.Stderr("PeersNeeded. Querying on background.")
			// The goroutine will write into the PeersNeededResults channel.
			go d.GetPeers(needPeers)
		case node := <-d.handshakeResults:
			if node != nil {
				d.nodes[node.id] = node
				if node.reachable {
					log.Stderr("reachable:", node.address)
				}
			} else {
				// Should never happen.
				log.Stderr("got a nil at d.RemoteNodeAcquaintance")
			}
		case <-d.Quit:
			log.Stderr("Exiting..")
			return
			//
			// case needMoreNodes := <-PeersNeeded // (torrent ran out of peers)
			// case newActiveTorrent := <-DudeWeHeardAboutANewTorrent
			//					       // We are tracking a new torrent.
			//                                             // Adverstise ourselfes as a node
			//                                             // for it.)
			// case reachableHostNotification := <-handshakeWait // but this is currently
			//                                                  // for each remote host..
		}
	}
}

// Should be run as goroutine. Caller must read results from d.PeersNeededResults.
func (d *DhtEngine) GetPeers(peers *InfohashPeers) {
	ih := peers.infoHash
	for _, r := range d.nodes {
		if !r.reachable {
			continue
		}
		// TODO: proper distance detection.
		peers.nodes = r.recursiveGetPeers(ih, 5)
		break // TODO: decide when to stop, and whether to start returning results earlier.
	}
	for k, _ := range peers.nodes {
		log.Stdoutf("Found TCP torrent peer candidate %q", k)
	}
	d.PeersNeededResults <- peers
}

// TODO: Create a routing table. Save routing table on disk to be preserved between instances.
// TODO: keep a blacklist of DHT nodes somewhere so we dont keep trying to connect to them.

// TODO: Fix dhtstats. Add channel to DHT engine that copies stats.
func dhtstats() string {
	// Not thread-safe but who cares, nobody uses this anyway.
	b, _ := json.MarshalIndent(&DhtStats, "", "\t")
	return string(b)
}

func init() {
	DhtStats.engines = make([]*DhtEngine, 1, 10)
	expvar.Publish("dhtengine", expvar.StringFunc(dhtstats))
}
