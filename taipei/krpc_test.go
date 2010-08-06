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

var pingTests = []pingTest{
	pingTest{"XX", "abcdefghij0123456789", "d1:ad2:id20:abcdefghij0123456789e1:q4:ping1:t2:XX1:y1:qe"},
}

func TestPing(t *testing.T) {
	for _, p := range pingTests {
		n, err := NewDhtNode(p.nodeId)
		if err != nil {
			t.Errorf("NewDhtNode(): %v", err)
		}
		r := newRemoteNode(n, "") // Address
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
	getPeersTest{"aa", "abcdefghij0123456789", "mnopqrstuvwxyz123456", "d1:ad2:id20:abcdefghij01234567899:info_hash20:mnopqrstuvwxyz123456e1:q9:get_peers1:t2:aa1:y1:q"},
}


func TestGetPeers(t *testing.T) {
	for _, p := range getPeersTests {
		n, err := NewDhtNode(p.nodeId)
		if err != nil {
			t.Errorf("NewDhtNode(): %v", err)
		}
		r := newRemoteNode(n, "") // Address
		v, _ := r.encodedGetPeers(p.transId, p.infoHash)
		if v != p.out {
			t.Errorf("GetPeers(%s, %s) = %s, want %s.", p.nodeId, p.infoHash, v, p.out)
		}
	}
}


// Requires Internet access.
func TestDht(t *testing.T) {
	node, err := NewDhtNode("abcdefghij0123456789")
	if err != nil {
		t.Errorf("NewDhtNode(): %v", err)
	}
	go node.DoDht()

	// Until we have our own DHT router, we'll use the bittorrent.com ones.
	// I hope they dont mind.. :-P.
	realDHTNodes := []string{
		//"127.0.0.1:33149",
		"router.bittorrent.com:6881",
	}
	for i, a := range realDHTNodes {
		t.Log("syn", i)
		node.RemoteNodeAcquaintance <- a
	}
	time.Sleep(1 * NS_PER_S)
	for _, k := range realDHTNodes {
		if _, ok := node.goodNodes[k]; !ok {
			t.Error("Node not in good nodes list", k)
		}
	}
	t.Log("Ordering DHT node to exit..")
	// If something went bad, this will block forever.
	node.Quit <- true
}
