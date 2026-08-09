package transfer_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/whaeuser/drop/internal/crypto"
	"github.com/whaeuser/drop/internal/p2p"
	"github.com/whaeuser/drop/internal/transfer"
)

func fakeSenderP2PFail(t *testing.T, conn *websocket.Conn, key, content []byte, filename string, sendOffer bool) {
	write := func(m transfer.Message) {
		ct, err := crypto.EncryptChunk(transfer.EncodeMessage(m), key)
		if err != nil {
			t.Fatal(err)
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, ct); err != nil {
			t.Fatal(err)
		}
	}

	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	if sendOffer {
		offerer, err := p2p.NewPeer(p2p.RoleOfferer, p2p.Config{})
		if err != nil {
			t.Fatal(err)
		}
		defer offerer.Close()
		sdp, err := offerer.CreateOffer(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		write(transfer.NewSDPOfferMessage(sdp))
		time.Sleep(200 * time.Millisecond)
	}

	write(transfer.NewP2PFailMessage("forced"))

	const chunk = 64 * 1024
	meta, err := transfer.NewMetadataMessage(filename, int64(len(content)), (len(content)+chunk-1)/chunk)
	if err != nil {
		t.Fatal(err)
	}
	write(meta)
	for off := 0; off < len(content); off += chunk {
		end := off + chunk
		if end > len(content) {
			end = len(content)
		}
		write(transfer.NewChunkMessage(content[off:end]))
	}
	write(transfer.NewCompleteMessage())
}

func runP2PFallback(t *testing.T, sendOffer bool) {
	relay := startTestRelay(t)
	defer relay.Close()
	relayURL := "ws" + relay.URL[4:]
	token := "p2pfail-test1234"

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	destDir := t.TempDir()
	content := bytes.Repeat([]byte("fallback "), 20000)
	filename := "fallback.bin"

	var wg sync.WaitGroup
	var recvPath string
	var recvErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		r := transfer.NewReceiver(relayURL, token, key)
		if err := r.ConnectLive(); err != nil {
			recvErr = err
			return
		}
		defer r.Close()
		recvPath, recvErr = r.ReceiveFile(destDir, nil)
	}()

	time.Sleep(100 * time.Millisecond)
	conn, _, err := websocket.DefaultDialer.Dial(relayURL+"/ws/"+token, nil)
	if err != nil {
		t.Fatal(err)
	}
	fakeSenderP2PFail(t, conn, key, content, filename, sendOffer)
	conn.Close()

	wg.Wait()
	if recvErr != nil {
		t.Fatalf("receive: %v", recvErr)
	}
	got, err := os.ReadFile(recvPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch: got %d bytes, want %d", len(got), len(content))
	}
	if filepath.Base(recvPath) != filename {
		t.Fatalf("filename: got %q want %q", filepath.Base(recvPath), filename)
	}
}

func TestReceiverFallback_P2PFailFirst(t *testing.T) {
	runP2PFallback(t, false)
}

func TestReceiverFallback_P2PFailAfterOffer(t *testing.T) {
	runP2PFallback(t, true)
}

// receiver hits its own NegotiateTimeout before any P2P_FAIL arrives (same-host race).
func TestReceiverFallback_OwnTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("waits for NegotiateTimeout")
	}

	relay := startTestRelay(t)
	defer relay.Close()
	relayURL := "ws" + relay.URL[4:]
	token := "p2ptimeout-test1"

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	destDir := t.TempDir()
	content := bytes.Repeat([]byte("timeout "), 8000)
	filename := "timeout.bin"

	var wg sync.WaitGroup
	var recvPath string
	var recvErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		r := transfer.NewReceiver(relayURL, token, key)
		if err := r.ConnectLive(); err != nil {
			recvErr = err
			return
		}
		defer r.Close()
		recvPath, recvErr = r.ReceiveFile(destDir, nil)
	}()

	time.Sleep(100 * time.Millisecond)
	conn, _, err := websocket.DefaultDialer.Dial(relayURL+"/ws/"+token, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	write := func(m transfer.Message) {
		ct, err := crypto.EncryptChunk(transfer.EncodeMessage(m), key)
		if err != nil {
			t.Fatal(err)
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, ct); err != nil {
			t.Fatal(err)
		}
	}
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	offerer, err := p2p.NewPeer(p2p.RoleOfferer, p2p.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer offerer.Close()
	sdp, err := offerer.CreateOffer(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	write(transfer.NewSDPOfferMessage(sdp))

	// wait past NegotiateTimeout without sending P2P_FAIL
	time.Sleep(transfer.NegotiateTimeout + time.Second)

	const chunk = 64 * 1024
	meta, err := transfer.NewMetadataMessage(filename, int64(len(content)), (len(content)+chunk-1)/chunk)
	if err != nil {
		t.Fatal(err)
	}
	write(meta)
	for off := 0; off < len(content); off += chunk {
		end := off + chunk
		if end > len(content) {
			end = len(content)
		}
		write(transfer.NewChunkMessage(content[off:end]))
	}
	write(transfer.NewCompleteMessage())

	wg.Wait()
	if recvErr != nil {
		t.Fatalf("receive: %v", recvErr)
	}
	got, err := os.ReadFile(recvPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch: got %d bytes, want %d", len(got), len(content))
	}
}
