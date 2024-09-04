package p2p

import (
	"bit_torrent_cli/peers"
)

const (
	MaxBlockSize = 16384
	MaxBacklog   = 5
)

type Torrent struct {
	Peers       []peers.Peer
	PeerID      [20]byte
	InfoHash    [20]byte
	PieceHashes [][20]byte
	PieceLength int
	Length      int
	Name        string
}

type pieceWord struct {
	index  int
	hash   [20]byte
	length int
}

type pieceResult struct {
	index int
	buf   []byte
}

type pieceProgress struct {
	index int
	// client *client.Client
}
