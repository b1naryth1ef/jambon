package tacview

import (
	"testing"
)

func testHash32(password string, expected string, t *testing.T) {
	hash := hashPassword32(password)
	if hash != expected {
		t.Fatalf("Hash32 mismatch for \"%s\"; expected %s, calculated %s.", password, expected, hash)
	}
}

func testHash64(password string, expected string, t *testing.T) {
	hash := hashPassword64(password)
	if hash != expected {
		t.Fatalf("Hash64 mismatch for \"%s\"; expected %s, calculated %s.", password, expected, hash)
	}
}

func TestHashPassword32(t *testing.T) {
	testHash32("",         "0",        t)
	testHash32("pass",     "7742b741", t)
	testHash32("password", "f335183e", t)
	testHash32("abc",      "ad957ab0", t)
	testHash32("abc123",   "223b140f", t)
	testHash32("12345",    "fcc50d33", t)
}

func TestHashPassword64(t *testing.T) {
	testHash64("",         "0",                t)
	testHash64("pass",     "5e1e445fd60ac2e0", t)
	testHash64("password", "3c0e55f1cfff14c4", t)
	testHash64("abc",      "fc99a9ae7dfa5bfc", t)
	testHash64("abc123",   "2bd464b05d7103f1", t)
	testHash64("12345",    "6b40207b495297f4", t)
}
