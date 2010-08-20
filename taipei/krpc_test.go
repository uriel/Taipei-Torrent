package taipei

import (
	"testing"
	"time"
)

type pingTest struct {
	transId string
	nodeId  string
	out     string
}

func startDhtNode(t *testing.T) *DhtEngine {
	node, err := NewDhtNode("abcdefghij0123456789")
	if err != nil {
		t.Errorf("NewDhtNode(): %v", err)
	}
	go node.DoDht()
	return node
}

var pingTests = []pingTest{
	pingTest{"XX", "abcdefghij0123456789", "d1:ad2:id20:abcdefghij0123456789e1:q4:ping1:t2:XX1:y1:qe"},
}

func TestPing(t *testing.T) {
	for _, p := range pingTests {
		node := startDhtNode(t)
		r := node.newRemoteNode("", "") // id, Address
		v, _ := r.encodedPing(p.transId)
		if v != p.out {
			t.Errorf("Ping(%s) = %s, want %s.", p.nodeId, v, p.out)
		}
	}
}

type getPeersTest struct {
	transId  string
	nodeId   string
	infoHash string
	out      string
}

var getPeersTests = []getPeersTest{
	getPeersTest{"aa", "abcdefghij0123456789", "mnopqrstuvwxyz123456", "d1:ad2:id20:abcdefghij01234567899:info_hash20:mnopqrstuvwxyz123456e1:q9:get_peers1:t2:aa1:y1:qe"},
}

func TestGetPeers(t *testing.T) {
	for _, p := range getPeersTests {
		n := startDhtNode(t)
		r := n.newRemoteNode("", "") // id, address
		v, _ := r.encodedGetPeers(p.transId, p.infoHash)
		if v != p.out {
			t.Errorf("GetPeers(%s, %s) = %s, want %s.", p.nodeId, p.infoHash, v, p.out)
		}
	}
}

// Requires Internet access.
func TestDhtBigAndSlow(t *testing.T) {
	node := startDhtNode(t)
	realDHTNodes := map[string]string{
		// DHT test router.
		"DHT_ROUTER": "dht.cetico.org:9660",
		//"DHT_ROUTER": "router.bittorrent.com:6881",
		//"DHT_ROUTER": "localhost:33149",
	}
	for id, address := range realDHTNodes {
		candidate := &DhtNodeCandidate{id: id, address: address}
		node.RemoteNodeAcquaintance <- candidate
	}
	time.Sleep(1.5 * UDP_READ_TIMEOUT) // ReadTimeout is set to 3
	for id, _ := range realDHTNodes {
		if address, ok := node.goodNodes[id]; !ok {
			t.Fatalf("External DHT node not reachable: %s", address)
		}
	}
	// Test the needPeers feature using an Ubuntu image.
	// http://releases.ubuntu.com/9.10/ubuntu-9.10-desktop-i386.iso.torrent
	infoHash := string([]byte{0x98, 0xc5, 0xc3, 0x61, 0xd0, 0xbe, 0x5f,
		0x2a, 0x07, 0xea, 0x8f, 0xa5, 0x05, 0x2e,
		0x5a, 0xa4, 0x80, 0x97, 0xe7, 0xf6})
	needPeers := node.NewNeedDhtPeers(infoHash)
	node.PeersNeeded <- needPeers
	needPeers = <-node.PeersNeededResults
	t.Logf("%d new torrent peers obtained.", len(needPeers.nodes))
	if len(needPeers.nodes) == 0 {
		t.Fatal("Could not find new torrent peers.")
	}

	// All went well!
	t.Logf("List of all reachable DHT hosts:")
	for _, r := range node.goodNodes {
		t.Log(r.address)
	}
	t.Log("Ordering DHT node to exit..")
	node.Quit <- true
}
