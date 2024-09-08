package p2p

import (
	"bit_torrent_cli/client"
	"bit_torrent_cli/message"
	"bit_torrent_cli/peers"
	"bytes"
	"crypto/sha1"
	"fmt"
	"log"
	"runtime"
	"time"
)

const (
	MaxBlockSize = 16384
	MaxBacklog   = 5
)

type Torrent struct {
	Name        string
	Peers       []peers.Peer
	PieceHashes [][20]byte
	PieceLength int
	Length      int
	PeerID      [20]byte
	InfoHash    [20]byte
}

type pieceWord struct {
	index  int
	hash   [20]byte
	length int
}

type pieceResult struct {
	buf   []byte
	index int
}

type pieceProgress struct {
	client     *client.Client
	buf        []byte
	index      int
	downloaded int
	requested  int
	backlog    int
}

func (state *pieceProgress) readMessage() error {
	msg, err := state.client.Read()
	if err != nil {
		return err
	}

	if msg == nil {
		return nil
	}

	switch msg.ID {
	case message.MsgUnchoke:
		state.client.Choked = false
	case message.MsgChoke:
		state.client.Choked = true
	case message.MsgHave:
		index, err := message.ParseHave(msg)
		if err != nil {
			return err
		}
		state.client.Bitfield.SetPiece(index)
	case message.MsgPiece:
		n, err := message.ParsePiece(state.index, state.buf, msg)
		if err != nil {
			return err
		}
		state.downloaded += n
		state.backlog--

	}
	return nil
}

// attenpDownloadPiece downloads a piece from a peer and returns the buffer
func attenpDownloadPiece(c *client.Client, pw *pieceWord) ([]byte, error) {
	state := pieceProgress{
		index:  pw.index,
		client: c,
		buf:    make([]byte, pw.length),
	}
	c.Conn.SetDeadline(time.Now().Add(30 * time.Second))
	defer c.Conn.SetDeadline(time.Time{})

	for state.downloaded < pw.length {
		if !state.client.Choked {
			for state.backlog < MaxBacklog && state.requested < pw.length {
				blockSize := MaxBlockSize
				if pw.length-state.requested < blockSize {
					blockSize = pw.length - state.requested
				}

				err := c.SendRequest(pw.index, state.requested, blockSize)
				if err != nil {
					return nil, err
				}
				state.backlog++
				state.requested += blockSize
			}
		}

		err := state.readMessage()
		if err != nil {
			return nil, err
		}
	}
	return state.buf, nil
}

// checkIntegrity checks the integrity of the downloaded piece
func checkIntegrity(pw *pieceWord, buf []byte) error {
	bash := sha1.Sum(buf)
	if !bytes.Equal(bash[:], pw.hash[:]) {
		return fmt.Errorf("index %d failed integrity check ", pw.index)
	}
	return nil
}

// Download starts the download of the torrent
func (t *Torrent) startDownLoadWorker(peer peers.Peer, workQueue chan *pieceWord, resultQueue chan *pieceResult) {
	c, err := client.New(peer, t.PeerID, t.InfoHash)
	if err != nil {
		log.Printf("cound not handshake with %s . disconnecting \n", peer.IP)
		return
	}
	defer c.Conn.Close()

	log.Printf(" completed handshake with %s\n", peer.IP)
	c.Sendunchoke()
	c.SendInterested()

	// send bitfield
	for pw := range workQueue {
		if !c.Bitfield.HasPiece(pw.index) {
			workQueue <- pw
			continue
		}

		buf, err := attenpDownloadPiece(c, pw)
		if err != nil {
			log.Println("Exiting", err)
			workQueue <- pw
			return
		}
		err = checkIntegrity(pw, buf)
		if err != nil {
			log.Printf("piece #%d failed integrity check \n", pw.index)
			workQueue <- pw
			continue
		}
		// send have message
		c.SendHave(pw.index)
		resultQueue <- &pieceResult{index: pw.index, buf: buf}
	}
}

// Download starts the download of the torrent
func (t *Torrent) calculateBoundsForPiece(index int) (begin, end int) {
	begin = index * t.PieceLength
	end = begin + t.PieceLength
	if end > t.Length {
		end = t.Length
	}
	return begin, end
}

// calculatePieceSize calculates the size of a piece
func (t *Torrent) calculatePieceSize(index int) int {
	begin, end := t.calculateBoundsForPiece(index)
	return end - begin
}

func (t *Torrent) Download() ([]byte, error) {
	log.Println("starting dowload for ", t.Name)
	workqueue := make(chan *pieceWord, len(t.PieceHashes))
	results := make(chan *pieceResult)
	for index, hash := range t.PieceHashes {
		length := t.calculatePieceSize(index)
		workqueue <- &pieceWord{index: index, hash: hash, length: length}
	}

	for _, peer := range t.Peers {
		go t.startDownLoadWorker(peer, workqueue, results)
	}

	buf := make([]byte, t.Length)
	donePieces := 0
	for donePieces < len(t.PieceHashes) {
		res := <-results
		begin, end := t.calculateBoundsForPiece(res.index)
		copy(buf[begin:end], res.buf)
		donePieces++
		percent := float64(donePieces) / float64(len(t.PieceHashes)) * 100
		numWorkers := runtime.NumGoroutine() - 1 // subtract 1 for main thread
		log.Printf("(%0.2f%%) Downloaded piece #%d from %d peers\n", percent, res.index, numWorkers)
	}
	close(workqueue)
	return buf, nil
}
