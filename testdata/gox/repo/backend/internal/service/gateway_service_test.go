package service

import "testing"

func TestForwardCountTokens(t *testing.T) {
	if ForwardCountTokens() == "" {
		t.Fatal("expected request path")
	}
}
