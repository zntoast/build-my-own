package torrentfile

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "update .golden.json  file")

func TestOpen(t *testing.T) {
	torrent, err := Open("C:/Users/user/Desktop/workspace/go/build-my-own/bit_torrent_cli/torrentfile/testdata/archlinux-2019.12.01-x86_64.iso.torrent")
	require.Nil(t, err)

	golddenPath := "C:/Users/user/Desktop/workspace/go/build-my-own/bit_torrent_cli/torrentfile/testdata/archlinux-2019.12.01-x86_64.iso.torrent.golden (1).json"
	if *update {
		serialized, err := json.MarshalIndent(torrent, "", " ")
		require.Nil(t, err)
		ioutil.WriteFile(golddenPath, serialized, 0644)
	}

	expected := Torrentfile{}
	golden, err := ioutil.ReadFile(golddenPath)
	require.Nil(t, err)
	json.Unmarshal(golden, &expected)
	require.Nil(t, err)

	assert.Equal(t, expected, torrent)
}

func TestTorrentFile(t *testing.T) {
}
