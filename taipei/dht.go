package taipei

import (
	"os"
)
// Owned by the DHT engine.
type DhtEngine struct {
	PeerID string
	// These are always known to be good nodes.
	nodes map[string]*DhtRemoteNode
	// Channels. Maybe expose only this to the torrent.
	// HelloDhtNode
	// DhtPeersNeeded
	// DudeWeHeardAboutANewTorrent
	handshakeResults chan *DhtRemoteNode
}

func NewDhtNode(nodeId string) (node *DhtEngine, err os.Error) {
	node = &DhtEngine{
		PeerID:           nodeId,
		nodes:            make(map[string]*DhtRemoteNode),
		handshakeResults: make(chan *DhtRemoteNode),
	}
	return
}

// DhtEngine contains channel for communication of DHT and torrent engines. DHT
// stuff should be separate from the rest of the torrent module. Maybe a
// separate module. Maybe allow it to run as a standalone router.
//
// Serves the high level real purpose of DHT.
// func DoDht() {
//
//     addressNewNode := <-HelloDhtNode  // (torrent heard about a new node)
//     needMoreNodes := <-DhtPeersNeeded // (torrent ran out of peers)
//     newActiveTorrent := <-DudeWeHeardAboutANewTorrent
//					       // We are tracking a new torrent.
//                                             // Adverstise ourselfes as a node
//                                             // for it.)
//     reachableHostNotification := <-handshakeWait // but this is currently
//                                                  // for each remote host..
// }
