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
	"github.com/whaeuser/drop/internal/transfer"
)

const discoverBrowseTimeout = 4 * time.Second

// cmdDiscover browses the LAN for active `drop send` sessions via mDNS and
// connects straight to the one the user picks — no passphrase prompt, since
// the advertised record already carries the room token and key (see
// internal/discovery for the trust boundary that implies). If discovery
// finds nothing, this points back at the always-available
// `drop download <passphrase>` fallback.
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

	chosen, err := selectSender(records, os.Stdin)
	if err != nil {
		return err
	}
	derivedToken, derivedKey := chosen.Token, chosen.Key
	fmt.Printf("  %s✓ Connecting to %s%s\n\n", colorGreen, chosen.Instance, colorReset)

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

// selectSender prompts (via in) for a menu choice among records and returns
// the chosen one directly — it already carries the token and key needed to
// connect.
func selectSender(records []discovery.ServiceRecord, in io.Reader) (discovery.ServiceRecord, error) {
	reader := bufio.NewReader(in)

	fmt.Printf("\n  Choose %s[1]%s: ", colorBrand, colorReset)
	choiceLine, _ := reader.ReadString('\n')
	idx, err := parseChoice(strings.TrimSpace(choiceLine), len(records))
	if err != nil {
		return discovery.ServiceRecord{}, err
	}
	return records[idx], nil
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
