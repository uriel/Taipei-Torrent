package taipei

import (
	"testing"
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
		r, err := NewRemoteNode(n, "") // Address
		if err != nil {
			t.Errorf("NewRemoteNode(): %v", err)
		}
		v, _ := r.ping(p.transId)
		if v != p.out {
			t.Errorf("Ping(%s) = %s, want %s.", p.nodeId, v, p.out)
		}
	}
}

// Requires Internet access.
func TestRemoteDht(t *testing.T) {
	node, err := NewDhtNode("abcdefghij0123456789")
	if err != nil {
		t.Errorf("NewDhtNode(): %v", err)
	}
	// Until we have our own DHT router, we'll use the bittorrent.com ones.
	// I hope they dont mind.. :-P.
	realDHTNodes := []string{
		// "127.0.0.1:53390",
		"router.bittorrent.com:6881",
	}
	for _, a := range realDHTNodes {
		r, _ := NewRemoteNode(node, a)
		if r.localNode != node {
			t.Errorf("Different localNodes %v ==> %v", node, r.localNode)
		}
	}
	for i := 0; i < len(realDHTNodes); i++ {
		if reachable := <-node.handshakeResults; reachable != nil {
			continue
		} else {
			t.Errorf("Node %v not reachable", reachable.address)
		}
	}
}
