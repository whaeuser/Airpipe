package transfer

import (
	"encoding/binary"
	"encoding/json"
	"errors"
)

type MessageType byte

const (
	MsgTypeMetadata      MessageType = 0x01
	MsgTypeReady         MessageType = 0x02
	MsgTypeComplete      MessageType = 0x03
	MsgTypeError         MessageType = 0x04
	MsgTypeChunk         MessageType = 0x10
	MsgTypeProgress      MessageType = 0x11
	MsgTypeVersion       MessageType = 0x20
	MsgTypeSDPOffer      MessageType = 0x30
	MsgTypeSDPAnswer     MessageType = 0x31
	MsgTypeICECandidate  MessageType = 0x32
	MsgTypeP2PReady      MessageType = 0x33
	MsgTypeP2PFail       MessageType = 0x34
	MsgTypePeerJoin      MessageType = 0x35
	MsgTypeSessionEnd    MessageType = 0x36
	MsgTypeResumeRequest MessageType = 0x37
)

// v4: resume support (ResumeRequest message + Metadata.ResumeOffset field) plus a
// version byte on PeerJoin for the live P2P path, which previously had no version
// handshake at all. A v3 peer doesn't understand ResumeRequest or the PeerJoin
// version byte, so mixed versions must fail closed on the PeerJoin check.
const ProtocolVersion byte = 4

type Metadata struct {
	Filename     string `json:"filename"`
	Size         int64  `json:"size"`
	Chunks       int    `json:"chunks"`
	ResumeOffset int64  `json:"resume_offset,omitempty"`
}

// ResumeRequest is sent by a receiver over the WS signaling connection, before
// PeerJoin, when it already holds a partial ".part" file for the transfer it's
// about to receive. Offset must be a multiple of ChunkSize; PrefixHash is the
// SHA-256 of the first Offset bytes already on disk, letting the sender confirm
// it's resuming the same file rather than an unrelated stale partial.
type ResumeRequest struct {
	Filename   string `json:"filename"`
	Offset     int64  `json:"offset"`
	PrefixHash string `json:"prefix_hash"`
}

type Progress struct {
	ChunkIndex  int   `json:"chunk_index"`
	TotalChunks int   `json:"total_chunks"`
	BytesSent   int64 `json:"bytes_sent"`
	TotalBytes  int64 `json:"total_bytes"`
}

type Message struct {
	Type    MessageType
	Payload []byte
}

func EncodeMessage(msg Message) []byte {
	result := make([]byte, 5+len(msg.Payload))
	result[0] = byte(msg.Type)
	binary.BigEndian.PutUint32(result[1:5], uint32(len(msg.Payload)))
	copy(result[5:], msg.Payload)
	return result
}

func DecodeMessage(data []byte) (Message, error) {
	if len(data) < 5 {
		return Message{}, errors.New("message too short")
	}
	msgType := MessageType(data[0])
	payloadLen := binary.BigEndian.Uint32(data[1:5])
	if len(data) < int(5+payloadLen) {
		return Message{}, errors.New("incomplete message")
	}
	return Message{
		Type:    msgType,
		Payload: data[5 : 5+payloadLen],
	}, nil
}

func NewMetadataMessage(filename string, size int64, chunks int) (Message, error) {
	return NewMetadataMessageWithResume(filename, size, chunks, 0)
}

// NewMetadataMessageWithResume is NewMetadataMessage plus a resumeOffset: the
// number of bytes at the start of the file the receiver already has and that
// the sender is skipping over. Pass 0 for a normal, non-resumed transfer.
func NewMetadataMessageWithResume(filename string, size int64, chunks int, resumeOffset int64) (Message, error) {
	meta := Metadata{Filename: filename, Size: size, Chunks: chunks, ResumeOffset: resumeOffset}
	payload, err := json.Marshal(meta)
	if err != nil {
		return Message{}, err
	}
	return Message{Type: MsgTypeMetadata, Payload: payload}, nil
}

func NewChunkMessage(data []byte) Message {
	return Message{Type: MsgTypeChunk, Payload: data}
}

func NewReadyMessage() Message {
	return Message{Type: MsgTypeReady, Payload: nil}
}

func NewCompleteMessage() Message {
	return Message{Type: MsgTypeComplete, Payload: nil}
}

func NewErrorMessage(errStr string) Message {
	return Message{Type: MsgTypeError, Payload: []byte(errStr)}
}

func NewVersionMessage() Message {
	return Message{Type: MsgTypeVersion, Payload: []byte{ProtocolVersion}}
}

func NewSDPOfferMessage(sdp string) Message {
	return Message{Type: MsgTypeSDPOffer, Payload: []byte(sdp)}
}

func NewSDPAnswerMessage(sdp string) Message {
	return Message{Type: MsgTypeSDPAnswer, Payload: []byte(sdp)}
}

func NewICECandidateMessage(candidate []byte) Message {
	return Message{Type: MsgTypeICECandidate, Payload: candidate}
}

func NewP2PReadyMessage() Message {
	return Message{Type: MsgTypeP2PReady, Payload: nil}
}

func NewP2PFailMessage(reason string) Message {
	return Message{Type: MsgTypeP2PFail, Payload: []byte(reason)}
}

// NewPeerJoinMessage carries the sender's protocol version so a mismatch on
// the live P2P path (which has no separate version handshake) fails closed
// the same way the mailbox path's MsgTypeVersion check does.
func NewPeerJoinMessage() Message {
	return Message{Type: MsgTypePeerJoin, Payload: []byte{ProtocolVersion}}
}

func NewSessionEndMessage() Message {
	return Message{Type: MsgTypeSessionEnd, Payload: nil}
}

// NewResumeRequestMessage is sent by a receiver, before PeerJoin, when it
// already holds a partial ".part" file it wants the sender to continue
// rather than restart.
func NewResumeRequestMessage(filename string, offset int64, prefixHash string) (Message, error) {
	req := ResumeRequest{Filename: filename, Offset: offset, PrefixHash: prefixHash}
	payload, err := json.Marshal(req)
	if err != nil {
		return Message{}, err
	}
	return Message{Type: MsgTypeResumeRequest, Payload: payload}, nil
}

func ParseResumeRequest(payload []byte) (ResumeRequest, error) {
	var req ResumeRequest
	err := json.Unmarshal(payload, &req)
	return req, err
}

func ParseMetadata(payload []byte) (Metadata, error) {
	var meta Metadata
	err := json.Unmarshal(payload, &meta)
	return meta, err
}
