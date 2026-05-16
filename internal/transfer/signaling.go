package transfer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/gorilla/websocket"
	"github.com/whaeuser/drop/internal/crypto"
	"github.com/whaeuser/drop/internal/p2p"
)

var ErrP2PFailed = errors.New("p2p failed, use ws fallback")

const NegotiateTimeout = 15 * time.Second

func writeSignalMsg(conn *websocket.Conn, key []byte, msg Message) error {
	encrypted, err := crypto.EncryptChunk(EncodeMessage(msg), key)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.BinaryMessage, encrypted)
}

func readSignalMsg(conn *websocket.Conn, key []byte) (Message, error) {
	_, raw, err := conn.ReadMessage()
	if err != nil {
		return Message{}, err
	}
	decrypted, err := crypto.DecryptChunk(raw, key)
	if err != nil {
		return Message{}, fmt.Errorf("decrypt signal: %w", err)
	}
	return DecodeMessage(decrypted)
}

type wsRead struct {
	msg Message
	err error
}

// gorilla/websocket panics on a read after a previous read error, so all reads
// funnel through this single goroutine.
func startWSReader(conn *websocket.Conn, key []byte, stopCh <-chan struct{}) <-chan wsRead {
	out := make(chan wsRead, 16)
	go func() {
		defer close(out)
		for {
			msg, err := readSignalMsg(conn, key)
			select {
			case out <- wsRead{msg: msg, err: err}:
			case <-stopCh:
				return
			}
			if err != nil {
				return
			}
		}
	}()
	return out
}

func negotiateSender(ctx context.Context, conn *websocket.Conn, key []byte) (*p2p.Peer, error) {
	negCtx, cancel := context.WithTimeout(ctx, NegotiateTimeout)
	defer cancel()

	peer, err := p2p.NewPeer(p2p.RoleOfferer, p2p.Config{})
	if err != nil {
		return nil, err
	}

	offer, err := peer.CreateOffer(negCtx)
	if err != nil {
		peer.Close()
		return nil, err
	}
	if err := writeSignalMsg(conn, key, NewSDPOfferMessage(offer)); err != nil {
		peer.Close()
		return nil, err
	}

	stopCh := make(chan struct{})
	defer close(stopCh)
	reads := startWSReader(conn, key, stopCh)

	trickleDone := make(chan struct{})
	go func() {
		defer close(trickleDone)
		for {
			select {
			case c, ok := <-peer.LocalICECandidates():
				if !ok {
					return
				}
				raw, _ := json.Marshal(c)
				_ = writeSignalMsg(conn, key, NewICECandidateMessage(raw))
			case <-negCtx.Done():
				return
			case <-peer.Closed():
				return
			}
		}
	}()

	openCh := make(chan struct{})
	go func() {
		_ = peer.WaitOpen(negCtx)
		close(openCh)
	}()

	for {
		select {
		case <-openCh:
			if !peer.IsOpen() {
				peer.Close()
				return nil, fmt.Errorf("datachannel did not open within %s", NegotiateTimeout)
			}
			cancel()
			<-trickleDone
			return peer, nil
		case r, ok := <-reads:
			if !ok {
				peer.Close()
				return nil, fmt.Errorf("signaling channel closed")
			}
			if r.err != nil {
				peer.Close()
				return nil, fmt.Errorf("read signal: %w", r.err)
			}
			switch r.msg.Type {
			case MsgTypeSDPAnswer:
				if err := peer.SetRemoteAnswer(negCtx, string(r.msg.Payload)); err != nil {
					peer.Close()
					return nil, fmt.Errorf("set remote answer: %w", err)
				}
			case MsgTypeICECandidate:
				_ = peer.AddICECandidate(r.msg.Payload)
			case MsgTypeP2PFail:
				peer.Close()
				return nil, ErrP2PFailed
			}
		case <-negCtx.Done():
			peer.Close()
			return nil, negCtx.Err()
		}
	}
}

func negotiateReceiver(ctx context.Context, conn *websocket.Conn, key []byte, offerSDP string) (*p2p.Peer, func() (Message, error), error) {
	negCtx, cancel := context.WithTimeout(ctx, NegotiateTimeout)

	peer, err := p2p.NewPeer(p2p.RoleAnswerer, p2p.Config{})
	if err != nil {
		cancel()
		return nil, nil, err
	}

	answer, err := peer.SetRemoteOffer(negCtx, offerSDP)
	if err != nil {
		peer.Close()
		cancel()
		return nil, nil, err
	}
	if err := writeSignalMsg(conn, key, NewSDPAnswerMessage(answer)); err != nil {
		peer.Close()
		cancel()
		return nil, nil, err
	}

	stopCh := make(chan struct{})
	reads := startWSReader(conn, key, stopCh)

	trickleDone := make(chan struct{})
	go func() {
		defer close(trickleDone)
		for {
			select {
			case c, ok := <-peer.LocalICECandidates():
				if !ok {
					return
				}
				raw, _ := json.Marshal(c)
				_ = writeSignalMsg(conn, key, NewICECandidateMessage(raw))
			case <-negCtx.Done():
				return
			case <-peer.Closed():
				return
			}
		}
	}()

	openCh := make(chan struct{})
	go func() {
		_ = peer.WaitOpen(negCtx)
		close(openCh)
	}()

	// Don't return until both the DC is open AND the sender has sent P2PReady.
	// Returning early lets the caller spawn a second WS reader, which gorilla
	// panics on.
	dcOpen, p2pReady := false, false
	for {
		if dcOpen && p2pReady {
			cancel()
			<-trickleDone
			close(stopCh)
			return peer, nil, nil
		}
		select {
		case <-openCh:
			dcOpen = true
		case r, ok := <-reads:
			if !ok {
				peer.Close()
				cancel()
				close(stopCh)
				return nil, nil, fmt.Errorf("signaling channel closed")
			}
			if r.err != nil {
				peer.Close()
				cancel()
				close(stopCh)
				return nil, nil, fmt.Errorf("read signal: %w", r.err)
			}
			switch r.msg.Type {
			case MsgTypeICECandidate:
				_ = peer.AddICECandidate(r.msg.Payload)
			case MsgTypeP2PReady:
				p2pReady = true
			case MsgTypeP2PFail:
				peer.Close()
				cancel()
				// Keep stopCh open so the reader goroutine stays alive.
				// Return a reader backed by the existing channel so no messages are lost.
				chanReader := func() (Message, error) {
					conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
					r, ok := <-reads
					if !ok {
						return Message{}, io.EOF
					}
					return r.msg, r.err
				}
				return nil, chanReader, ErrP2PFailed
			}
		case <-negCtx.Done():
			peer.Close()
			cancel()
			close(stopCh)
			return nil, nil, negCtx.Err()
		}
	}
}
