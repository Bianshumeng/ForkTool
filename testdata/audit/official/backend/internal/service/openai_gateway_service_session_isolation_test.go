package service

import "testing"

func TestOpenAISessionIsolation(t *testing.T) {
	if isolateOpenAISessionID("session") == "" {
		t.Fatal("expected isolated session")
	}
}
