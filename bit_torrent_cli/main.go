package main

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/url"
	"strconv"

	"github.com/jackpal/bencode-go"
)

type BencodeInfo struct {
	Pieces      string `bencode:"pieces"`
	PieceLength int    `bencode:"piece length"`
	Length      int    `bencode:"length"`
	Name        string `bencode:"name"`
}

type BencodeTorrent struct {
	Announce string `bencode:"announce"`
	Info     BencodeInfo
}

func Open(r io.Reader) (*BencodeTorrent, error) {
	bto := BencodeTorrent{}
	err := bencode.Unmarshal(r, &bto)
	if err != nil {
		return nil, err
	}
	return &bto, nil
}

type TorrendFile struct {
	Announce    string
	InfoHash    [20]byte
	PieceHashes [][20]byte
	PieceLength int
	Length      int
	Name        string
}

func (bto BencodeTorrent) toTorrenfFile() (TorrendFile, error) {
	return TorrendFile{}, nil
}

func (t *TorrendFile) buildTrackerURL(peerID [20]byte, port uint16) (string, error) {
	base, err := url.Parse(t.Announce)
	if err != nil {
		return "", err
	}
	params := url.Values{
		"info_hash": []string{string(t.InfoHash[:])},
		"peer_id":   []string{string(peerID[:])},
		"port":      []string{strconv.Itoa(int(port))},
		"uploaded":  []string{"0"},
		"download":  []string{"0"},
		"compact":   []string{"1"},
		"left":      []string{strconv.Itoa(t.Length)},
	}
	base.RawQuery = params.Encode()
	return base.String(), nil
}

type Peer struct {
	IP   net.IP
	Port uint16
}

func (p Peer) string() string {
	return net.JoinHostPort(p.IP.String(), strconv.Itoa(int(p.Port)))
}

// Unmarshal parses peer IP addresses and ports from a buffer
func Unmarshal(peersBin []byte) ([]Peer, error) {
	const peerSize = 6
	numPeers := len(peersBin) / peerSize
	if len(peersBin)%peerSize != 0 {
		return nil, errors.New("received malformed peers")
	}
	peers := make([]Peer, numPeers)
	for i := 0; i < numPeers; i++ {
		offset := i * peerSize
		peers[i].IP = net.IP(peersBin[offset : offset+4])
		peers[i].Port = binary.BigEndian.Uint16(peersBin[offset+4 : offset+6])
	}
	return peers, nil
}

func main() {

}

type messageID uint8

const (
	MsgChoke messageID = iota
	MsgUnchoke
	MsgInterested
	MsgNotInterested
	MsgHave
	MsgBitfield
	MsgRequest
	MsgPiece
	MsgCancel
)

// Message is a message sent between peers.
type Message struct {
	ID      messageID
	Payload []byte
}

// Serialize serializes the message into a byte slice.
// The first 4 bytes are the length of the payload, in big-endian order.
// The next byte is the message ID.
// The remaining bytes are the payload.
func (m *Message) Serialize() []byte {
	if m == nil {
		return make([]byte, 4)
	}
	length := uint32(len(m.Payload) + 1)
	buf := make([]byte, 4+len(m.Payload))
	binary.BigEndian.PutUint32(buf, length)
	buf[4] = byte(m.ID)
	copy(buf[5:], m.Payload)
	return buf
}

// Read reads a message from the reader.
func Read(r io.Reader) (*Message, error) {
	lengthBuf := make([]byte, 4)
	_, err := io.ReadFull(r, lengthBuf)
	if err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lengthBuf)

	if length == 0 {
		return nil, nil
	}

	messageBuf := make([]byte, length)
	_, err = io.ReadFull(r, messageBuf)
	if err != nil {
		return nil, err
	}
	id := messageID(messageBuf[0])
	_, err = io.ReadFull(r, messageBuf)
	if err != nil {
		return nil, err
	}

	m := Message{ID: id, Payload: messageBuf[1:]}
	return &m, nil
}
