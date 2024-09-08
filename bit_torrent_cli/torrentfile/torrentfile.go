package torrentfile

import (
	"bit_torrent_cli/p2p"
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"os"

	"github.com/jackpal/bencode-go"
)

// Port to listen on
const Port uint16 = 6881

type Torrentfile struct {
	Announce    string
	Name        string
	PieceHashes [][20]byte
	PieceLength int
	Length      int
	Infohash    [20]byte
}

// 定义种子文件的结构体
type Torrent struct {
	Announce     string   `bencode:"announce"`
	Info         Info     `bencode:"info"`
	CreationDate int64    `bencode:"creation date,omitempty"`
	Comment      string   `bencode:"comment,omitempty"`
	URLList      []string `bencode:"url-list,omitempty"`
}

type Info struct {
	PieceLength int    `bencode:"piece length"`
	Pieces      string `bencode:"pieces"`
	Files       []File `bencode:"files,omitempty"`
	Name        string `bencode:"name,omitempty"`
	Length      int    `bencode:"length,omitempty"`
}

type File struct {
	Length int      `bencode:"length"`
	Path   []string `bencode:"path"`
}

func (t *Torrentfile) DownloadToFile(path string) error {
	var peerID [20]byte
	_, err := rand.Read(peerID[:])
	if err != nil {
		return err
	}
	peers, err := t.requestPeers(peerID, Port)
	if err != nil {
		return err
	}
	torrent := p2p.Torrent{
		Peers:       peers,
		PeerID:      peerID,
		InfoHash:    t.Infohash,
		PieceHashes: t.PieceHashes,
		PieceLength: t.PieceLength,
		Length:      t.Length,
		Name:        t.Name,
	}
	buf, err := torrent.Download()
	if err != nil {
		return err
	}
	outfile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer outfile.Close()
	_, err = outfile.Write(buf)
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
	bto := Torrent{}
	err = bencode.Unmarshal(file, &bto)
	if err != nil {
		return Torrentfile{}, err
	}
	return bto.toTorrentFile()
}

func (i *Info) hash() ([20]byte, error) {
	var buf bytes.Buffer
	err := bencode.Marshal(&buf, *i)
	if err != nil {
		return [20]byte{}, err
	}
	h := sha1.Sum(buf.Bytes())
	return h, nil
}

func (i *Info) splitPieceHashes() ([][20]byte, error) {
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

func (bto *Torrent) toTorrentFile() (Torrentfile, error) {
	infoHash, err := bto.Info.hash()
	if err != nil {
		return Torrentfile{}, err
	}
	pieceHases, err := bto.Info.splitPieceHashes()
	if err != nil {
		return Torrentfile{}, err
	}
	t := Torrentfile{
		Announce:    bto.URLList[19],
		Infohash:    infoHash,
		PieceHashes: pieceHases,
		PieceLength: bto.Info.PieceLength,
		Length:      bto.Info.Length,
		Name:        bto.Info.Name,
	}
	return t, nil
}
