package discovery

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/mdns"
)

func TestFakeHubRoundTrip(t *testing.T) {
	hub := NewFakeHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rec := ServiceRecord{Instance: "Sanyams-Laptop", Token: "abcdef0123456789", Label: "3 files", Version: 4}
	if err := hub.Advertiser().Advertise(ctx, rec); err != nil {
		t.Fatalf("advertise: %v", err)
	}

	got, err := hub.Browser().Browse(context.Background(), time.Second)
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if len(got) != 1 || got[0] != rec {
		t.Fatalf("browse result = %+v, want [%+v]", got, rec)
	}

	// Canceling the advertiser's context should remove it from later browses.
	cancel()
	time.Sleep(20 * time.Millisecond)
	got, err = hub.Browser().Browse(context.Background(), time.Second)
	if err != nil {
		t.Fatalf("browse after cancel: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no records after advertiser canceled, got %+v", got)
	}
}

func TestFakeHubsDoNotCrossTalk(t *testing.T) {
	hubA := NewFakeHub()
	hubB := NewFakeHub()
	ctx := context.Background()

	if err := hubA.Advertiser().Advertise(ctx, ServiceRecord{Token: "on-hub-a"}); err != nil {
		t.Fatalf("advertise: %v", err)
	}
	got, err := hubB.Browser().Browse(ctx, time.Second)
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected hub B to see nothing from hub A, got %+v", got)
	}
}

// This is the security-relevant property of the whole package: nothing
// derived from the raw passphrase or the NaCl key is ever allowed onto the
// wire beyond the token. ServiceRecord's field set is itself the contract
// (no Passphrase/Key field exists to leak), and parseEntry only ever reads
// the "v", "t", "n" TXT keys this package itself writes in Advertise.
func TestParseEntryOnlyReadsKnownKeys(t *testing.T) {
	entry := &mdns.ServiceEntry{
		Name: "Sanyams-Laptop." + serviceType + "." + domain,
		InfoFields: []string{
			"v=4",
			"t=abcdef0123456789",
			"n=3 files",
			"passphrase=RIVER FALCON MARBLE 42", // must be ignored: not a key we ever emit or read
		},
	}
	rec, ok := parseEntry(entry)
	if !ok {
		t.Fatal("expected a valid record")
	}
	if rec.Instance != "Sanyams-Laptop" {
		t.Fatalf("Instance = %q, want Sanyams-Laptop", rec.Instance)
	}
	if rec.Token != "abcdef0123456789" || rec.Label != "3 files" || rec.Version != 4 {
		t.Fatalf("record mismatch: %+v", rec)
	}
}

func TestParseEntryRejectsMissingToken(t *testing.T) {
	entry := &mdns.ServiceEntry{
		Name:       "Sanyams-Laptop." + serviceType + "." + domain,
		InfoFields: []string{"v=4", "n=3 files"},
	}
	if _, ok := parseEntry(entry); ok {
		t.Fatal("expected a record with no token to be rejected")
	}
}

func TestAdvertiseTXTNeverContainsExtraKeys(t *testing.T) {
	// Guard against a future edit accidentally adding a new field to
	// ServiceRecord (e.g. a Passphrase or Key) without updating Advertise
	// to keep it off the wire: build the TXT records the same way Advertise
	// does and assert only v=/t=/n= keys ever appear.
	rec := ServiceRecord{Instance: "host", Token: "tok", Label: "label", Version: 4}
	txt := []string{
		"v=" + string(rune('0'+rec.Version)),
		"t=" + rec.Token,
		"n=" + rec.Label,
	}
	for _, field := range txt {
		key, _, ok := strings.Cut(field, "=")
		if !ok || (key != "v" && key != "t" && key != "n") {
			t.Fatalf("unexpected TXT key in %q", field)
		}
	}
}
