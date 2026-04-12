package svcimpl

import "testing"

func TestSessionRequestLooksConsistent_SameSubnet(t *testing.T) {
	if !sessionRequestLooksConsistent("192.168.1", "mozilla/5.0 chrome/120", "192.168.1.99", "Mozilla/5.0 Chrome/120") {
		t.Fatal("expected same /24 and UA to match")
	}
}

func TestSessionRequestLooksConsistent_IPChange(t *testing.T) {
	if sessionRequestLooksConsistent("10.0.0", "mozilla/5.0", "192.168.1.1", "Mozilla/5.0") {
		t.Fatal("expected different /24 to fail")
	}
}
