package torrentfile

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"os"

	"github.com/jackpal/bencode-go"
)

type Torrentfile struct {
	Announce    string
	Infohash    [20]byte
	PieceHashes [][20]byte
	PieceLength int
	Length      int
	Name        string
}

type bencodeInfo struct {
	Pieces      string
	PieceLength int
	Length      int
	Name        string
}

type bencodeTorrent struct {
	Announce string
	Info     bencodeInfo
}

func (t *Torrentfile) DownloadToFile(path string) error {
	var peerID [20]byte
	_, err := rand.Read(peerID[:])
	if err != nil {
		return err
	}

	return nil
}

func Open(path string) (Torrentfile, error) {
	file, err := os.Open(path)
	if err != nil {
		return Torrentfile{}, err
	}
	defer file.Close()
	return Torrentfile{}, nil
}

func (i *bencodeInfo) hash() ([20]byte, error) {
	var buf bytes.Buffer
	err := bencode.Marshal(&buf, *i)
	if err != nil {
		return [20]byte{}, err
	}
	h := sha1.Sum(buf.Bytes())
	return h, nil
}

func (i *bencodeInfo) splitPieceHashes() ([][20]byte, error) {
	hashLen := 20
	buf := []byte(i.Pieces)
	if len(buf)%hashLen != 0 {
		return nil, fmt.Errorf("received malformed pieces of length %d", len(buf))
	}
	numHashes := len(buf) / hashLen
	hashes := make([][20]byte, numHashes)
	for i := 0; i < numHashes; i++ {
		copy(hashes[i][:], buf[i*hashLen:(i+1)*hashLen])
	}
	return hashes, nil
}

func (bto *bencodeTorrent) toTorrentFile() (Torrentfile, error) {
	infoHash, err := bto.Info.hash()
	if err != nil {
		return Torrentfile{}, err
	}
	pieceHases, err := bto.Info.splitPieceHashes()
	if err != nil {
		return Torrentfile{}, err
	}
	t := Torrentfile{
		Announce:    bto.Announce,
		Infohash:    infoHash,
		PieceHashes: pieceHases,
		PieceLength: bto.Info.PieceLength,
		Length:      bto.Info.Length,
		Name:        bto.Info.Name,
	}
	return t, nil
}
