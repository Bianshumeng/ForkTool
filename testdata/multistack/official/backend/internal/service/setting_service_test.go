package service

import "testing"

func TestUpdateStatusPageURL(t *testing.T) {
	if UpdateStatusPageURL("https://status.example.com") == "" {
		t.Fatal("expected status page url")
	}
}
