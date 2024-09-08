package handshake

import (
	"fmt"
	"io"
)

// Handshake is the initial message sent by a client to initiate a connection.
type Handshake struct {
	Pstr     string
	InfoHash [20]byte
	PeerID   [20]byte
}

func New(infoHash, peerID [20]byte) *Handshake {
	return &Handshake{
		Pstr:     "BitTorrent protocol",
		InfoHash: infoHash,
		PeerID:   peerID,
	}
}

// Serialize serializes the handshake into a byte slice.
func (h *Handshake) Serialize() []byte {
	pstrlen := len(h.Pstr)
	bufLen := 49 + pstrlen
	buf := make([]byte, bufLen)
	buf[0] = byte(pstrlen) // pstrlen
	copy(buf[1:], h.Pstr)  // pstr
	copy(buf[1+pstrlen+8:], h.InfoHash[:])
	copy(buf[1+pstrlen+8+20:], h.PeerID[:])
	return buf
}

// Read reads a handshake from a reader.
func Read(r io.Reader) (*Handshake, error) {
	lengthBuf := make([]byte, 1)
	_, err := io.ReadFull(r, lengthBuf)
	if err != nil {
		return nil, err
	}
	pstrLen := int(lengthBuf[0])

	if pstrLen == 0 {
		err := fmt.Errorf("pstelen cannot be zero")
		return nil, err
	}

	// Read the rest of the handshake
	handshakeBuf := make([]byte, 48+pstrLen)
	_, err = io.ReadFull(r, handshakeBuf)
	if err != nil {
		return nil, err
	}

	var infoHash, peerID [20]byte
	copy(infoHash[:], handshakeBuf[pstrLen+8:pstrLen+8+20])
	copy(peerID[:], handshakeBuf[pstrLen+8+20:])

	h := &Handshake{
		Pstr:     string(handshakeBuf[:pstrLen]),
		InfoHash: infoHash,
		PeerID:   peerID,
	}
	return h, nil
}
