package discovery

import (
	"context"
	"sync"
	"time"
)

// FakeHub is an in-process stand-in for the LAN multicast domain, letting
// send/discover logic be exercised in tests without real mDNS sockets.
type FakeHub struct {
	mu      sync.Mutex
	nextID  int
	records map[int]ServiceRecord
}

// NewFakeHub returns an empty hub. Advertisers and Browsers obtained from
// the same hub see each other's records; hubs never cross-talk.
func NewFakeHub() *FakeHub { return &FakeHub{records: make(map[int]ServiceRecord)} }

func (h *FakeHub) Advertiser() Advertiser { return fakeAdvertiser{hub: h} }
func (h *FakeHub) Browser() Browser       { return fakeBrowser{hub: h} }

type fakeAdvertiser struct{ hub *FakeHub }

func (f fakeAdvertiser) Advertise(ctx context.Context, rec ServiceRecord) error {
	f.hub.mu.Lock()
	id := f.hub.nextID
	f.hub.nextID++
	f.hub.records[id] = rec
	f.hub.mu.Unlock()
	go func() {
		<-ctx.Done()
		f.hub.mu.Lock()
		delete(f.hub.records, id)
		f.hub.mu.Unlock()
	}()
	return nil
}

type fakeBrowser struct{ hub *FakeHub }

func (f fakeBrowser) Browse(ctx context.Context, timeout time.Duration) ([]ServiceRecord, error) {
	f.hub.mu.Lock()
	defer f.hub.mu.Unlock()
	out := make([]ServiceRecord, 0, len(f.hub.records))
	for _, rec := range f.hub.records {
		out = append(out, rec)
	}
	return out, nil
}
