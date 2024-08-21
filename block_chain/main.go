package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strconv"
	"time"
)

type Block struct {
	Timestamp     int64
	Data          []byte
	PrevBlockHash []byte
	Hash          []byte
}

func (b *Block) SetHash() {
	timestamp := []byte(strconv.FormatInt(b.Timestamp, 10))
	headers := bytes.Join([][]byte{b.PrevBlockHash, b.Data, timestamp}, []byte(""))
	hash := sha256.Sum256(headers)
	b.Hash = hash[:]
}

func NewBlcok(data string, prevBlockHash []byte) *Block {
	block := &Block{
		Timestamp:     time.Now().Unix(),
		Data:          []byte(data),
		PrevBlockHash: prevBlockHash,
		Hash:          []byte{},
	}
	block.SetHash()
	return block
}

type BlockChain struct {
	Blocks []*Block
}

func (bc *BlockChain) AddBlock(data string) {
	prevBlock := bc.Blocks[len(bc.Blocks)-1]
	newBlock := NewBlcok(data, prevBlock.Hash)
	bc.Blocks = append(bc.Blocks, newBlock)
}

func NewGenesisBlock() *Block {
	return NewBlcok("Genesis Block", []byte{})
}

func NewBlcokChain() *BlockChain {
	bc := &BlockChain{Blocks: []*Block{NewGenesisBlock()}}
	return bc
}

func main() {
	bc := NewBlcokChain()
	bc.AddBlock("Send 1 BTC to Ivan")
	bc.AddBlock("Send 2 more BTC to Ivan")
	for _, block := range bc.Blocks {
		fmt.Printf("Prev . hash : %x\n ", block.PrevBlockHash)
		fmt.Printf("data .   %s\n ", block.Data)
		fmt.Printf("hash .   %x\n ", block.Hash)
		fmt.Println()
	}
}
