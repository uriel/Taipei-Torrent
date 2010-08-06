// DHT node for Taipei Torrent, for tracker-less peer information exchange.
package taipei

import (
	"log"
	"os"
)

// DhtEngine should be created by NewDhtNode() and provides DHT features like
// finding new peers for torrent downloads, witout requiring a tracker. The
// client can only use the public (first letter uppercase) channels for
// communicating with the DHT goroutines. 
type DhtEngine struct {
	peerID string
	// Currently only keep good ones in memory. 
	goodNodes              map[string]*DhtRemoteNode
	// DHT internal channel.
	handshakeResults       chan *DhtRemoteNode

	// Public channels:
	Quit                   chan bool
	RemoteNodeAcquaintance chan string
	// DhtPeersNeeded
	// DudeWeHeardAboutANewTorrent
}

func NewDhtNode(nodeId string) (node *DhtEngine, err os.Error) {
	node = &DhtEngine{
		peerID:                 nodeId,
		goodNodes:              make(map[string]*DhtRemoteNode),
		handshakeResults:       make(chan *DhtRemoteNode),
		RemoteNodeAcquaintance: make(chan string),
		Quit:                   make(chan bool),
	}
	return
}

// DoDht starts the DHT node and should be run as a goroutine. To make it quit,
// send any value to the Quit channel of the DhtEngine.
func (d *DhtEngine) DoDht() {
	for {
		select {
		case helloNode := <-d.RemoteNodeAcquaintance:
			// We've got a new node id. We need to:
			// - ping it and see if it's reachable. Ignore otherwise.
			// - save it on our list of good nodes.
			// - later, we'll implement bucketing, etc.
			newRemoteNode(d, helloNode)
		case reachable := <-d.handshakeResults:
			if reachable != nil {
				log.Stderr("reachable:", reachable.address)
				d.goodNodes[reachable.address] = reachable
			} else {
				// Should never happen.
				log.Stderr("got a nil at d.RemoteNodeAcquaintance")
			}
		case <-d.Quit:
			log.Stderr("Exiting..")
			return
			//
			// case needMoreNodes := <-DhtPeersNeeded // (torrent ran out of peers)
			// case newActiveTorrent := <-DudeWeHeardAboutANewTorrent
			//					       // We are tracking a new torrent.
			//                                             // Adverstise ourselfes as a node
			//                                             // for it.)
			// case reachableHostNotification := <-handshakeWait // but this is currently
			//                                                  // for each remote host..
		}
	}
}

// TODO: Create a routing table. Save routing table on disk to be preserved between instances.
