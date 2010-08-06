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
		//"127.0.0.1:53390",
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
