package taipei

import (
	//"taipei"
	"testing"
	)

type pingTest struct {
	in string // ignored
	out string
}

var pingTests = []pingTest{
	pingTest{"abcdefghij0123456789", "d1:ad2:id20:abcdefghij0123456789e1:q4:ping1:t2:aa1:y1:qe"},
}

func TestDouble(t *testing.T) {
	for _, p := range pingTests {
		n := &DhtNode{p.in}
		v, _ := n.Ping()
		if v != p.out {
			t.Errorf("Double(%d) = %d, want %d.", p.in, v, p.out)
		}
	}
}
