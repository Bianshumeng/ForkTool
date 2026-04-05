package service

import "testing"

func TestUpdateStatusPageURL(t *testing.T) {
	if UpdateStatusPageURL("https://local.example.com") == "" {
		t.Fatal("expected status page url")
	}
}
