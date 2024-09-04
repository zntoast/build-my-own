package client

import (
	"bit_torrent_cli/bitfield"
	"bit_torrent_cli/handshake"
	"bit_torrent_cli/peers"
	"bytes"
	"fmt"
	"net"
	"time"
)

type Client struct {
	Conn     net.Conn
	Choked   bool
	Bitfield bitfield.Bitfield
	peer     peers.Peer
	infoHash [20]byte
	peerID   [20]byte
}

func completeHandshake(conn net.Conn, infohash, peerID [20]byte) (*handshake.Handshake, error) {
	conn.SetDeadline(time.Now().Add(time.Second * 3))
	defer conn.SetDeadline(time.Time{}) // disable the deadline
	req := handshake.New(infohash, peerID)
	_, err := conn.Write(req.Serialize())
	if err != nil {
		return nil, err
	}

	res, err := handshake.Read(conn)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(res.InfoHash[:], infohash[:]) {
		return nil, fmt.Errorf("expected infohas %v but got %x ", res.InfoHash, infohash)
	}
	return res, nil
}

// func recvBitfield(conn net.Conn) (bitfield.Bitfield, error) {
// 	conn.SetDeadline(time.Now().Add(time.Second * 5))
// 	defer conn.SetDeadline(time.Time{})
// }
