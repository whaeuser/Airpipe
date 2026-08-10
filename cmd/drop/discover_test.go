package main

import (
	"context"
	"strings"
	"testing"

	"github.com/whaeuser/drop/internal/discovery"
	"github.com/whaeuser/drop/internal/passphrase"
)

func TestSelectSenderReturnsChosenRecord(t *testing.T) {
	phrase := "RIVER FALCON MARBLE 42"
	key := passphrase.DeriveKey(phrase)
	records := []discovery.ServiceRecord{
		{Instance: "Erics-Desktop", Token: "some-other-token", Key: []byte("decoy-key-0000000000000000000000")},
		{Instance: "Sanyams-Laptop", Token: passphrase.DeriveToken(phrase), Key: key[:]},
	}

	in := strings.NewReader("2\n")
	chosen, err := selectSender(records, in)
	if err != nil {
		t.Fatalf("selectSender failed: %v", err)
	}
	if chosen.Instance != "Sanyams-Laptop" {
		t.Fatalf("chosen = %+v, want Sanyams-Laptop", chosen)
	}
	if chosen.Token != passphrase.DeriveToken(phrase) {
		t.Fatalf("token mismatch: got %q", chosen.Token)
	}
	if string(chosen.Key) != string(key[:]) {
		t.Fatal("key mismatch")
	}
}

func TestSelectSenderDefaultsToFirstChoice(t *testing.T) {
	records := []discovery.ServiceRecord{
		{Instance: "OnlySender", Token: "sometoken", Key: []byte("k")},
	}
	in := strings.NewReader("\n") // empty line -> default choice 1
	chosen, err := selectSender(records, in)
	if err != nil {
		t.Fatalf("selectSender failed: %v", err)
	}
	if chosen.Instance != "OnlySender" {
		t.Fatalf("chosen = %+v, want OnlySender", chosen)
	}
}

func TestSelectSenderOutOfRangeChoiceRejected(t *testing.T) {
	records := []discovery.ServiceRecord{{Instance: "X", Token: "sometoken", Key: []byte("k")}}
	in := strings.NewReader("5\n")
	if _, err := selectSender(records, in); err == nil {
		t.Fatal("expected an error for an out-of-range menu choice")
	}
}

func TestParseChoice(t *testing.T) {
	if idx, err := parseChoice("", 3); err != nil || idx != 0 {
		t.Fatalf("empty choice: idx=%d err=%v, want 0 nil", idx, err)
	}
	if idx, err := parseChoice("2", 3); err != nil || idx != 1 {
		t.Fatalf("choice 2: idx=%d err=%v, want 1 nil", idx, err)
	}
	if _, err := parseChoice("0", 3); err == nil {
		t.Fatal("expected error for choice 0")
	}
	if _, err := parseChoice("4", 3); err == nil {
		t.Fatal("expected error for out-of-range choice")
	}
	if _, err := parseChoice("nope", 3); err == nil {
		t.Fatal("expected error for non-numeric choice")
	}
}

// End-to-end sanity that discovery.FakeHub + selectSender compose: a sender
// advertises on a fake hub, a receiver browses the same hub and correctly
// picks it out among a decoy, getting back the token+key needed to connect
// with no further prompt.
func TestDiscoverFakeHubIntegration(t *testing.T) {
	hub := discovery.NewFakeHub()
	phrase := "OCEAN TIGER STORM 77"
	key := passphrase.DeriveKey(phrase)
	ctx := context.Background()

	if err := hub.Advertiser().Advertise(ctx, discovery.ServiceRecord{
		Instance: "Decoy", Token: "decoy-token-0000", Key: []byte("decoy-key-000000000000000000000"),
	}); err != nil {
		t.Fatalf("advertise decoy: %v", err)
	}
	if err := hub.Advertiser().Advertise(ctx, discovery.ServiceRecord{
		Instance: "RealSender", Token: passphrase.DeriveToken(phrase), Key: key[:], Label: "1 file",
	}); err != nil {
		t.Fatalf("advertise real sender: %v", err)
	}

	records, err := hub.Browser().Browse(ctx, 0)
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	// Pick whichever index is "RealSender" and verify its token+key came
	// through untouched.
	var wantIdx int
	for i, r := range records {
		if r.Instance == "RealSender" {
			wantIdx = i + 1
		}
	}
	if wantIdx == 0 {
		t.Fatal("RealSender not found in browse results")
	}

	in := strings.NewReader(itoa(wantIdx) + "\n")
	chosen, err := selectSender(records, in)
	if err != nil {
		t.Fatalf("selectSender: %v", err)
	}
	if chosen.Instance != "RealSender" {
		t.Fatalf("chosen = %+v, want RealSender", chosen)
	}
	if chosen.Token != passphrase.DeriveToken(phrase) {
		t.Fatalf("token mismatch: got %q", chosen.Token)
	}
	if string(chosen.Key) != string(key[:]) {
		t.Fatal("key mismatch")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
