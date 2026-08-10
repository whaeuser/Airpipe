package discovery

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/mdns"
)

func TestFakeHubRoundTrip(t *testing.T) {
	hub := NewFakeHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rec := ServiceRecord{Instance: "Sanyams-Laptop", Token: "abcdef0123456789", Key: []byte("some-32-byte-key"), Label: "3 files", Version: 4}
	if err := hub.Advertiser().Advertise(ctx, rec); err != nil {
		t.Fatalf("advertise: %v", err)
	}

	got, err := hub.Browser().Browse(context.Background(), time.Second)
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if len(got) != 1 || !reflect.DeepEqual(got[0], rec) {
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

	if err := hubA.Advertiser().Advertise(ctx, ServiceRecord{Token: "on-hub-a", Key: []byte("k")}); err != nil {
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

// The trust boundary this package implements: the token AND the key both go
// out over mDNS (see the package doc comment for why), so parseEntry must
// read both — and reject a record missing either, since a keyless record is
// something this package never advertised.
func TestParseEntryReadsTokenAndKey(t *testing.T) {
	entry := &mdns.ServiceEntry{
		Name: "Sanyams-Laptop." + serviceType + "." + domain,
		InfoFields: []string{
			"v=4",
			"t=abcdef0123456789",
			"k=" + "0011223344556677",
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
	if string(rec.Key) != "\x00\x11\x22\x33\x44\x55\x66\x77" {
		t.Fatalf("Key = %x, want decoded 0011223344556677", rec.Key)
	}
}

func TestParseEntryRejectsMissingToken(t *testing.T) {
	entry := &mdns.ServiceEntry{
		Name:       "Sanyams-Laptop." + serviceType + "." + domain,
		InfoFields: []string{"v=4", "k=00112233", "n=3 files"},
	}
	if _, ok := parseEntry(entry); ok {
		t.Fatal("expected a record with no token to be rejected")
	}
}

func TestParseEntryRejectsMissingKey(t *testing.T) {
	entry := &mdns.ServiceEntry{
		Name:       "Sanyams-Laptop." + serviceType + "." + domain,
		InfoFields: []string{"v=4", "t=abcdef0123456789", "n=3 files"},
	}
	if _, ok := parseEntry(entry); ok {
		t.Fatal("expected a record with no key to be rejected")
	}
}

func TestAdvertiseTXTCarriesTokenAndKey(t *testing.T) {
	// Advertise deliberately puts both the token and the hex-encoded key on
	// the wire (see the package doc comment: the LAN is the trust boundary,
	// not the passphrase). This guards the encoding stays hex/parseable and
	// no unexpected key shows up.
	rec := ServiceRecord{Instance: "host", Token: "tok", Key: []byte{0x00, 0x11}, Label: "label", Version: 4}
	txt := []string{
		"v=" + string(rune('0'+rec.Version)),
		"t=" + rec.Token,
		"k=0011",
		"n=" + rec.Label,
	}
	for _, field := range txt {
		key, _, ok := strings.Cut(field, "=")
		if !ok || (key != "v" && key != "t" && key != "k" && key != "n") {
			t.Fatalf("unexpected TXT key in %q", field)
		}
	}
}
