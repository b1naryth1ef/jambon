package tacview

import (
	"testing"
)

func testHash(password string, expected uint64, t *testing.T) {
	hash := hashPassword(password)
	if hash != expected {
		t.Fatalf("Hash mismatch for \"%s\"; expected %x, calculated %x.", password, expected, hash)
	}
}

func TestHashPassword(t *testing.T) {
	testHash("",         0x0000000000000000, t)
	testHash("pass",     0x5e1e445fd60ac2e0, t)
	testHash("password", 0x3c0e55f1cfff14c4, t)
	testHash("abc",      0xfc99a9ae7dfa5bfc, t)
	testHash("abc123",   0x2bd464b05d7103f1, t)
	testHash("12345",    0x6b40207b495297f4, t)
}
