package agent1234

import "testing"

func TestRandomBitwiseXor(t *testing.T) {
	const a, b uint8 = 0x5a, 0xa5
	if a^b != 0xff {
		t.Fatalf("expected 0xff, got %#x", a^b)
	}
}

// hello world
