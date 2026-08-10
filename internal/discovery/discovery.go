// Package discovery lets a `drop send` narrow down which sender a receiver
// on the same LAN is looking for, via mDNS. It is purely a UX shortcut in
// front of the existing relay-signaled transfer: it never carries the
// passphrase or the encryption key, and a failure here never blocks or
// fails a transfer — `drop download <passphrase>` keeps working exactly as
// before regardless of whether discovery succeeds.
package discovery

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
)

const (
	serviceType = "_drop._tcp"
	domain      = "local."

	// dummyPort is required by the mDNS library's SRV record but nothing
	// ever listens on it: the actual transfer always goes through the
	// existing relay/WebRTC path, never a direct connection to this port.
	dummyPort = 7862
)

// ServiceRecord is what a sender advertises and a receiver discovers on the
// LAN. Only the derived room token goes out over multicast — never the
// passphrase or the NaCl key. The token alone lets a listener find/join the
// relay-signaled WS room, but it can't decrypt anything without the key,
// which comes from a separate SHA-256 domain (see internal/passphrase) —
// so broadcasting it on the LAN costs nothing.
type ServiceRecord struct {
	Instance string // human-readable label, defaults to hostname
	Token    string // passphrase.DeriveToken(...) output
	Label    string // optional cosmetic hint (filename, file count) - never secret
	Version  byte   // transfer.ProtocolVersion at advertise time
}

// Advertiser broadcasts a ServiceRecord on the LAN until ctx is canceled.
type Advertiser interface {
	Advertise(ctx context.Context, rec ServiceRecord) error
}

// Browser looks for ServiceRecords being advertised on the LAN.
type Browser interface {
	Browse(ctx context.Context, timeout time.Duration) ([]ServiceRecord, error)
}

// NewAdvertiser returns the real mDNS-backed Advertiser.
func NewAdvertiser() Advertiser { return mdnsAdvertiser{} }

// NewBrowser returns the real mDNS-backed Browser.
func NewBrowser() Browser { return mdnsBrowser{} }

type mdnsAdvertiser struct{}

func (mdnsAdvertiser) Advertise(ctx context.Context, rec ServiceRecord) error {
	instance := rec.Instance
	if instance == "" {
		instance, _ = os.Hostname()
	}
	if instance == "" {
		instance = "drop"
	}

	txt := []string{
		fmt.Sprintf("v=%d", rec.Version),
		fmt.Sprintf("t=%s", rec.Token),
	}
	if rec.Label != "" {
		txt = append(txt, fmt.Sprintf("n=%s", rec.Label))
	}

	svc, err := mdns.NewMDNSService(instance, serviceType, domain, "", dummyPort, nil, txt)
	if err != nil {
		return fmt.Errorf("mdns service: %w", err)
	}
	server, err := mdns.NewServer(&mdns.Config{Zone: svc})
	if err != nil {
		return fmt.Errorf("mdns server: %w", err)
	}
	go func() {
		<-ctx.Done()
		server.Shutdown()
	}()
	return nil
}

type mdnsBrowser struct{}

func (mdnsBrowser) Browse(ctx context.Context, timeout time.Duration) ([]ServiceRecord, error) {
	entriesCh := make(chan *mdns.ServiceEntry, 16)
	var records []ServiceRecord
	done := make(chan struct{})
	go func() {
		defer close(done)
		for e := range entriesCh {
			if rec, ok := parseEntry(e); ok {
				records = append(records, rec)
			}
		}
	}()

	err := mdns.QueryContext(ctx, &mdns.QueryParam{
		Service: serviceType,
		Domain:  strings.TrimSuffix(domain, "."),
		Timeout: timeout,
		Entries: entriesCh,
	})
	close(entriesCh)
	<-done
	if err != nil {
		return nil, fmt.Errorf("mdns query: %w", err)
	}
	return records, nil
}

func parseEntry(e *mdns.ServiceEntry) (ServiceRecord, bool) {
	suffix := "." + serviceType + "." + domain
	rec := ServiceRecord{Instance: strings.TrimSuffix(e.Name, suffix)}

	for _, field := range e.InfoFields {
		key, val, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		switch key {
		case "v":
			if len(val) == 1 {
				rec.Version = val[0] - '0'
			}
		case "t":
			rec.Token = val
		case "n":
			rec.Label = val
		}
	}
	// A record without a token is useless (and never one we advertised).
	if rec.Token == "" {
		return ServiceRecord{}, false
	}
	return rec, true
}
