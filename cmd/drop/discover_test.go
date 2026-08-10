package main

import (
	"context"
	"strings"
	"testing"

	"github.com/whaeuser/drop/internal/discovery"
	"github.com/whaeuser/drop/internal/passphrase"
)

func TestSelectSenderCorrectPassphrase(t *testing.T) {
	phrase := "RIVER FALCON MARBLE 42"
	records := []discovery.ServiceRecord{
		{Instance: "Erics-Desktop", Token: "wrong-token-aaaa"},
		{Instance: "Sanyams-Laptop", Token: passphrase.DeriveToken(phrase)},
	}

	in := strings.NewReader("2\n" + phrase + "\n")
	chosen, token, key, err := selectSender(records, in)
	if err != nil {
		t.Fatalf("selectSender failed: %v", err)
	}
	if chosen.Instance != "Sanyams-Laptop" {
		t.Fatalf("chosen = %+v, want Sanyams-Laptop", chosen)
	}
	if token != passphrase.DeriveToken(phrase) {
		t.Fatalf("token mismatch: got %q", token)
	}
	wantKey := passphrase.DeriveKey(phrase)
	if len(key) != 32 || string(key) != string(wantKey[:]) {
		t.Fatal("derived key mismatch")
	}
}

func TestSelectSenderDefaultsToFirstChoice(t *testing.T) {
	phrase := "OCEAN TIGER STORM 77"
	records := []discovery.ServiceRecord{
		{Instance: "OnlySender", Token: passphrase.DeriveToken(phrase)},
	}
	in := strings.NewReader("\n" + phrase + "\n") // empty line -> default choice 1
	chosen, _, _, err := selectSender(records, in)
	if err != nil {
		t.Fatalf("selectSender failed: %v", err)
	}
	if chosen.Instance != "OnlySender" {
		t.Fatalf("chosen = %+v, want OnlySender", chosen)
	}
}

// The whole point of local verification: a wrong passphrase for the chosen
// sender must be rejected before any network connection is attempted, and
// it must never be confused with a *different* advertised sender's token.
func TestSelectSenderWrongPassphraseRejected(t *testing.T) {
	records := []discovery.ServiceRecord{
		{Instance: "Sanyams-Laptop", Token: passphrase.DeriveToken("RIVER FALCON MARBLE 42")},
	}
	in := strings.NewReader("1\nTOTALLY WRONG PHRASE 99\n")
	if _, _, _, err := selectSender(records, in); err == nil {
		t.Fatal("expected an error for a passphrase that doesn't match the chosen record's token")
	}
}

func TestSelectSenderEmptyPassphraseRejected(t *testing.T) {
	records := []discovery.ServiceRecord{{Instance: "X", Token: "sometoken"}}
	in := strings.NewReader("1\n\n")
	if _, _, _, err := selectSender(records, in); err == nil {
		t.Fatal("expected an error for an empty passphrase")
	}
}

func TestSelectSenderOutOfRangeChoiceRejected(t *testing.T) {
	records := []discovery.ServiceRecord{{Instance: "X", Token: "sometoken"}}
	in := strings.NewReader("5\nirrelevant\n")
	if _, _, _, err := selectSender(records, in); err == nil {
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
// picks it out among a decoy.
func TestDiscoverFakeHubIntegration(t *testing.T) {
	hub := discovery.NewFakeHub()
	phrase := "OCEAN TIGER STORM 77"
	ctx := context.Background()

	if err := hub.Advertiser().Advertise(ctx, discovery.ServiceRecord{
		Instance: "Decoy", Token: "decoy-token-0000",
	}); err != nil {
		t.Fatalf("advertise decoy: %v", err)
	}
	if err := hub.Advertiser().Advertise(ctx, discovery.ServiceRecord{
		Instance: "RealSender", Token: passphrase.DeriveToken(phrase), Label: "1 file",
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

	// Pick whichever index is "RealSender" and verify against its passphrase.
	var wantIdx int
	for i, r := range records {
		if r.Instance == "RealSender" {
			wantIdx = i + 1
		}
	}
	if wantIdx == 0 {
		t.Fatal("RealSender not found in browse results")
	}

	in := strings.NewReader(itoa(wantIdx) + "\n" + phrase + "\n")
	chosen, _, _, err := selectSender(records, in)
	if err != nil {
		t.Fatalf("selectSender: %v", err)
	}
	if chosen.Instance != "RealSender" {
		t.Fatalf("chosen = %+v, want RealSender", chosen)
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
