package main

import (
	"bit_torrent_cli/torrentfile"
	"fmt"
	"log"
	"os"
)

func main() {
	inPath := os.Args[1]
	outPath := os.Args[2]
	tf, err := torrentfile.Open(inPath)
	if err != nil {
		log.Fatal(err)
		fmt.Printf("err: %v\n", err)
		return
	}
	err = tf.DownloadToFile(outPath)
	if err != nil {
		log.Fatal(err)
		fmt.Printf("err: %v\n", err)
	}
}
