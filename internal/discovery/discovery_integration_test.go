//go:build integration

package discovery

import (
	"context"
	"testing"
	"time"
)

// TestRealMDNSRoundTrip exercises actual multicast UDP sockets end to end.
// It's excluded from the default `go test ./...` run (needs a real network
// stack with multicast enabled, which CI sandboxes and some containers
// don't allow) — run explicitly with `go test -tags integration ./internal/discovery/...`.
func TestRealMDNSRoundTrip(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rec := ServiceRecord{Instance: "integration-test-host", Token: "0123456789abcdef", Version: 4}
	if err := NewAdvertiser().Advertise(ctx, rec); err != nil {
		t.Fatalf("advertise: %v", err)
	}

	// Give the mDNS server a moment to bind before querying.
	time.Sleep(200 * time.Millisecond)

	found, err := NewBrowser().Browse(context.Background(), 3*time.Second)
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	for _, r := range found {
		if r.Token == rec.Token {
			return
		}
	}
	t.Fatalf("advertised record not found via real mDNS browse; got %+v", found)
}
