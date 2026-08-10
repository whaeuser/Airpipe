// Package discovery lets a `drop send` narrow down which sender a receiver
// on the same LAN is looking for, via mDNS. It is purely a UX shortcut in
// front of the existing relay-signaled transfer, and a failure here never
// blocks or fails a transfer — `drop download <passphrase>` keeps working
// exactly as before regardless of whether discovery succeeds.
//
// The advertised record includes the encryption key itself, so anyone who
// can see the mDNS multicast (i.e. anyone on the same LAN/WiFi segment) can
// pick the sender from `drop discover` and receive without ever being told
// the passphrase. This is a deliberate trust boundary: the LAN itself is
// treated as the shared secret, not the passphrase. `drop download
// <passphrase>` remains the option when that's not an acceptable trade-off
// (untrusted/guest network segments that still relay mDNS).
package discovery

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
)

// silentLogger swallows the mdns library's own stderr chatter (defaults to
// log.Default() otherwise). It's noisy in ways that don't reflect real
// failures here — e.g. it logs "Failed to listen ... on IPv6" whenever IPv6
// wasn't attempted at all, which is always the case since we pass
// DisableIPv6 below. Actual failures are reported through this package's
// own return values, which is what callers should act on.
var silentLogger = log.New(io.Discard, "", 0)

const (
	serviceType = "_drop._tcp"
	domain      = "local."

	// dummyPort is required by the mDNS library's SRV record but nothing
	// ever listens on it: the actual transfer always goes through the
	// existing relay/WebRTC path, never a direct connection to this port.
	dummyPort = 7862
)

// ServiceRecord is what a sender advertises and a receiver discovers on the
// LAN: the room token and the NaCl key, hex-encoded, so a receiver can
// connect straight from the `drop discover` menu with no passphrase prompt.
type ServiceRecord struct {
	Instance string // human-readable label, defaults to hostname
	Token    string // passphrase.DeriveToken(...) output
	Key      []byte // passphrase.DeriveKey(...) output, 32 bytes
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
		fmt.Sprintf("k=%s", hex.EncodeToString(rec.Key)),
	}
	if rec.Label != "" {
		txt = append(txt, fmt.Sprintf("n=%s", rec.Label))
	}

	svc, err := mdns.NewMDNSService(instance, serviceType, domain, "", dummyPort, nil, txt)
	if err != nil {
		return fmt.Errorf("mdns service: %w", err)
	}
	server, err := mdns.NewServer(&mdns.Config{Zone: svc, Logger: silentLogger})
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
		// IPv6 multicast is routinely broken on client machines (VPNs, some
		// routers/Docker networks) in ways IPv4 isn't. The mdns library's
		// sendQuery aborts the whole query on the first send error, so a
		// single "no route to host" on the IPv6 send discards an already
		// in-flight, otherwise-working IPv4 query. IPv4 alone covers the
		// LAN-discovery use case this package exists for.
		DisableIPv6: true,
		Logger:      silentLogger,
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
		case "k":
			if key, err := hex.DecodeString(val); err == nil {
				rec.Key = key
			}
		case "n":
			rec.Label = val
		}
	}
	// A record without a token or key is useless (and never one we advertised).
	if rec.Token == "" || len(rec.Key) == 0 {
		return ServiceRecord{}, false
	}
	return rec, true
}
