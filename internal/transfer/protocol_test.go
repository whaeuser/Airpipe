package transfer

import (
	"bytes"
	"testing"
)

func TestEncodeDecodeRoundtrip(t *testing.T) {
	tests := []struct {
		name    string
		msgType MessageType
		payload []byte
	}{
		{"metadata", MsgTypeMetadata, []byte(`{"filename":"test.txt","size":100,"chunks":2}`)},
		{"chunk", MsgTypeChunk, []byte("file data here")},
		{"ready", MsgTypeReady, nil},
		{"complete", MsgTypeComplete, nil},
		{"error", MsgTypeError, []byte("something broke")},
		{"empty chunk", MsgTypeChunk, []byte{}},
		{"large payload", MsgTypeChunk, bytes.Repeat([]byte("x"), 64*1024)},
		{"session_end", MsgTypeSessionEnd, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := Message{Type: tt.msgType, Payload: tt.payload}
			encoded := EncodeMessage(msg)
			decoded, err := DecodeMessage(encoded)
			if err != nil {
				t.Fatalf("DecodeMessage failed: %v", err)
			}
			if decoded.Type != tt.msgType {
				t.Fatalf("type mismatch: got %d, want %d", decoded.Type, tt.msgType)
			}
			if !bytes.Equal(decoded.Payload, tt.payload) {
				t.Fatalf("payload mismatch")
			}
		})
	}
}

func TestDecodeMessageTooShort(t *testing.T) {
	_, err := DecodeMessage([]byte{0x01, 0x00})
	if err == nil {
		t.Fatal("expected error for short message")
	}
}

func TestDecodeMessageIncomplete(t *testing.T) {
	// Header says 100 bytes but only 5 bytes total
	data := []byte{0x01, 0x00, 0x00, 0x00, 0x64}
	_, err := DecodeMessage(data)
	if err == nil {
		t.Fatal("expected error for incomplete message")
	}
}

func TestNewMetadataMessage(t *testing.T) {
	msg, err := NewMetadataMessage("photo.jpg", 1024, 1)
	if err != nil {
		t.Fatalf("NewMetadataMessage failed: %v", err)
	}
	if msg.Type != MsgTypeMetadata {
		t.Fatalf("expected MsgTypeMetadata, got %d", msg.Type)
	}

	meta, err := ParseMetadata(msg.Payload)
	if err != nil {
		t.Fatalf("ParseMetadata failed: %v", err)
	}
	if meta.Filename != "photo.jpg" || meta.Size != 1024 || meta.Chunks != 1 {
		t.Fatalf("metadata mismatch: %+v", meta)
	}
}

func TestNewChunkMessage(t *testing.T) {
	data := []byte("chunk data")
	msg := NewChunkMessage(data)
	if msg.Type != MsgTypeChunk {
		t.Fatalf("expected MsgTypeChunk")
	}
	if !bytes.Equal(msg.Payload, data) {
		t.Fatal("payload mismatch")
	}
}

func TestNewReadyMessage(t *testing.T) {
	msg := NewReadyMessage()
	if msg.Type != MsgTypeReady {
		t.Fatalf("expected MsgTypeReady")
	}
}

func TestNewCompleteMessage(t *testing.T) {
	msg := NewCompleteMessage()
	if msg.Type != MsgTypeComplete {
		t.Fatalf("expected MsgTypeComplete")
	}
}

func TestNewErrorMessage(t *testing.T) {
	msg := NewErrorMessage("failed")
	if msg.Type != MsgTypeError {
		t.Fatalf("expected MsgTypeError")
	}
	if string(msg.Payload) != "failed" {
		t.Fatalf("expected 'failed', got %q", string(msg.Payload))
	}
}

func TestNewSessionEndMessage(t *testing.T) {
	msg := NewSessionEndMessage()
	if msg.Type != MsgTypeSessionEnd {
		t.Fatalf("expected MsgTypeSessionEnd")
	}
}

func TestParseMetadataInvalid(t *testing.T) {
	_, err := ParseMetadata([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestNewVersionMessage(t *testing.T) {
	msg := NewVersionMessage()
	if msg.Type != MsgTypeVersion {
		t.Fatalf("expected MsgTypeVersion, got %d", msg.Type)
	}
	if len(msg.Payload) != 1 || msg.Payload[0] != ProtocolVersion {
		t.Fatalf("expected payload [%d], got %v", ProtocolVersion, msg.Payload)
	}

	// Roundtrip
	encoded := EncodeMessage(msg)
	decoded, err := DecodeMessage(encoded)
	if err != nil {
		t.Fatalf("roundtrip failed: %v", err)
	}
	if decoded.Type != MsgTypeVersion || decoded.Payload[0] != ProtocolVersion {
		t.Fatalf("roundtrip mismatch: %+v", decoded)
	}
}

func TestNewResumeRequestMessageRoundtrip(t *testing.T) {
	msg, err := NewResumeRequestMessage("photo.jpg", 128*1024, "deadbeef")
	if err != nil {
		t.Fatalf("NewResumeRequestMessage failed: %v", err)
	}
	if msg.Type != MsgTypeResumeRequest {
		t.Fatalf("expected MsgTypeResumeRequest, got %d", msg.Type)
	}

	encoded := EncodeMessage(msg)
	decoded, err := DecodeMessage(encoded)
	if err != nil {
		t.Fatalf("roundtrip decode failed: %v", err)
	}

	req, err := ParseResumeRequest(decoded.Payload)
	if err != nil {
		t.Fatalf("ParseResumeRequest failed: %v", err)
	}
	if req.Filename != "photo.jpg" || req.Offset != 128*1024 || req.PrefixHash != "deadbeef" {
		t.Fatalf("resume request mismatch: %+v", req)
	}
}

func TestNewMetadataMessageWithResume(t *testing.T) {
	msg, err := NewMetadataMessageWithResume("photo.jpg", 4096, 4, 2048)
	if err != nil {
		t.Fatalf("NewMetadataMessageWithResume failed: %v", err)
	}
	meta, err := ParseMetadata(msg.Payload)
	if err != nil {
		t.Fatalf("ParseMetadata failed: %v", err)
	}
	if meta.ResumeOffset != 2048 {
		t.Fatalf("expected ResumeOffset 2048, got %d", meta.ResumeOffset)
	}
}

// A pre-v4 sender never sets resume_offset; decoding its Metadata JSON must
// default ResumeOffset to 0 rather than erroring, so old<->new interop degrades
// to "just start over" instead of breaking.
func TestParseMetadataBackwardCompatMissingResumeOffset(t *testing.T) {
	oldShapeJSON := []byte(`{"filename":"photo.jpg","size":1024,"chunks":1}`)
	meta, err := ParseMetadata(oldShapeJSON)
	if err != nil {
		t.Fatalf("ParseMetadata failed on old-shape JSON: %v", err)
	}
	if meta.ResumeOffset != 0 {
		t.Fatalf("expected ResumeOffset 0 for old-shape JSON, got %d", meta.ResumeOffset)
	}
	if meta.Filename != "photo.jpg" || meta.Size != 1024 || meta.Chunks != 1 {
		t.Fatalf("metadata mismatch: %+v", meta)
	}
}

func TestNewPeerJoinMessageCarriesVersion(t *testing.T) {
	msg := NewPeerJoinMessage()
	if msg.Type != MsgTypePeerJoin {
		t.Fatalf("expected MsgTypePeerJoin, got %d", msg.Type)
	}
	if len(msg.Payload) != 1 || msg.Payload[0] != ProtocolVersion {
		t.Fatalf("expected payload [%d], got %v", ProtocolVersion, msg.Payload)
	}
}

func TestEncodeMessageHeaderFormat(t *testing.T) {
	payload := []byte("test")
	msg := Message{Type: MsgTypeChunk, Payload: payload}
	encoded := EncodeMessage(msg)

	if encoded[0] != byte(MsgTypeChunk) {
		t.Fatal("first byte should be message type")
	}
	// payload length in big-endian: 4 bytes
	if encoded[1] != 0 || encoded[2] != 0 || encoded[3] != 0 || encoded[4] != 4 {
		t.Fatalf("length encoding wrong: %v", encoded[1:5])
	}
}
