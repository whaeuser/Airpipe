package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/whaeuser/drop/internal/discovery"
	"github.com/whaeuser/drop/internal/passphrase"
	"github.com/whaeuser/drop/internal/transfer"
)

const discoverBrowseTimeout = 4 * time.Second

// cmdDiscover browses the LAN for active `drop send` sessions via mDNS and
// lets the user pick one instead of typing/scanning a passphrase blind. It
// narrows down *which* sender to talk to; the passphrase is still required
// and is verified locally (DeriveToken match) before any connection is
// attempted — see internal/discovery for why the passphrase itself never
// goes out over mDNS. If discovery finds nothing, this points back at the
// always-available `drop download <passphrase>` fallback.
func cmdDiscover(relay string, args []string) error {
	stayOpen, rest := parseStayOpenFlags(args)
	destDir := "."
	if len(rest) >= 1 {
		destDir = rest[0]
	}

	banner("discover")
	fmt.Print("  Looking for senders on the LAN...")

	records, err := discovery.NewBrowser().Browse(context.Background(), discoverBrowseTimeout)
	if err != nil {
		fmt.Printf("\r  %sLAN discovery unavailable: %v%s%s\n\n", colorDim, err, colorReset, strings.Repeat(" ", 10))
		return fallbackHint()
	}
	if len(records) == 0 {
		fmt.Printf("\r  %sNo senders found on this network.%s%s\n\n", colorDim, colorReset, strings.Repeat(" ", 10))
		return fallbackHint()
	}

	fmt.Printf("\r  Found %d sender(s):%s\n\n", len(records), strings.Repeat(" ", 20))
	for i, r := range records {
		label := r.Instance
		if r.Label != "" {
			label += "  " + colorDim + "(" + r.Label + ")" + colorReset
		}
		fmt.Printf("    %s[%d]%s %s\n", colorBrand, i+1, colorReset, label)
	}

	_, derivedToken, derivedKey, err := selectSender(records, os.Stdin)
	if err != nil {
		return err
	}
	fmt.Printf("  %s✓ Passphrase verified%s\n\n", colorGreen, colorReset)

	wsRelay := toWS(relay)
	receiver := transfer.NewReceiver(wsRelay, derivedToken, derivedKey)
	if err := receiver.ConnectLive(); err != nil {
		return fmt.Errorf("connect to relay: %w", err)
	}
	defer receiver.Close()

	if stayOpen {
		err := receiver.ReceiveBatches(destDir, func(batchIdx int, paths []string) error {
			if batchIdx > 0 {
				fmt.Printf("\n  %s--- batch %d ---%s\n", colorDim, batchIdx+1, colorReset)
			}
			for _, savedPath := range paths {
				fmt.Printf("\n  %s✓ Saved: %s%s\n", colorGreen, savedPath, colorReset)
			}
			return nil
		}, func(_ int, received, total int64) {
			progress(received, total)
		})
		if err != nil {
			return fmt.Errorf("p2p receive: %w", err)
		}
		fmt.Println()
		return nil
	}

	savedPaths, err := receiver.ReceiveFiles(destDir, func(_ int, received, total int64) {
		progress(received, total)
	})
	if err != nil {
		return fmt.Errorf("p2p receive: %w", err)
	}
	for _, savedPath := range savedPaths {
		fmt.Printf("\n  %s✓ Saved: %s%s\n", colorGreen, savedPath, colorReset)
	}
	fmt.Println()
	return nil
}

func fallbackHint() error {
	fmt.Printf("  Ask the sender for their passphrase and use %sdrop download%s instead.\n\n", colorBold, colorReset)
	return nil
}

// selectSender prompts (via in) for a menu choice among records, then a
// passphrase for the chosen one, and verifies DeriveToken(phrase) matches
// the record's advertised token before returning — this is the only place
// discovery's mDNS-carried token gets cross-checked against a real
// passphrase, so a wrong guess or a stale/spoofed record is rejected
// locally, before any network connection is attempted.
func selectSender(records []discovery.ServiceRecord, in io.Reader) (discovery.ServiceRecord, string, []byte, error) {
	reader := bufio.NewReader(in)

	fmt.Printf("\n  Choose %s[1]%s: ", colorBrand, colorReset)
	choiceLine, _ := reader.ReadString('\n')
	idx, err := parseChoice(strings.TrimSpace(choiceLine), len(records))
	if err != nil {
		return discovery.ServiceRecord{}, "", nil, err
	}
	chosen := records[idx]

	fmt.Printf("  Passphrase for %s%s%s: ", colorBold, chosen.Instance, colorReset)
	phraseLine, _ := reader.ReadString('\n')
	phrase := strings.TrimSpace(phraseLine)
	if phrase == "" {
		return discovery.ServiceRecord{}, "", nil, fmt.Errorf("no passphrase entered")
	}

	derivedToken := passphrase.DeriveToken(phrase)
	if derivedToken != chosen.Token {
		return discovery.ServiceRecord{}, "", nil, fmt.Errorf("that passphrase doesn't match %s — double check and try again", chosen.Instance)
	}
	keyArr := passphrase.DeriveKey(phrase)
	return chosen, derivedToken, keyArr[:], nil
}

// parseChoice parses a 1-based menu selection; an empty string defaults to 1.
func parseChoice(s string, n int) (int, error) {
	if s == "" {
		s = "1"
	}
	idx, err := strconv.Atoi(s)
	if err != nil || idx < 1 || idx > n {
		return 0, fmt.Errorf("invalid choice: %q", s)
	}
	return idx - 1, nil
}
