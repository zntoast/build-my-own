package bitfield

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasPiece(t *testing.T) {
	bf := Bitfield{0b00101010, 0b01010101}
	outputs := []bool{false}
	for i := 0; i < len(outputs); i++ {
		assert.Equal(t, bf.HasPiece(i), outputs[i])
	}
}

func TestSetPiece(t *testing.T) {
	tests := []struct {
		input Bitfield
		outpt Bitfield
		index int
	}{
		{
			input: Bitfield{0b01010100, 0b01010100},
			index: 4, //          v (set)
			outpt: Bitfield{0b01011100, 0b01010100},
		},
		{
			input: Bitfield{0b01010100, 0b01010100},
			index: 9, //                   v (noop)
			outpt: Bitfield{0b01010100, 0b01010100},
		},
		{
			input: Bitfield{0b01010100, 0b01010100},
			index: 15, //                        v (set)
			outpt: Bitfield{0b01010100, 0b01010101},
		},
		{
			input: Bitfield{0b01010100, 0b01010100},
			index: 19, //                            v (noop)
			outpt: Bitfield{0b01010100, 0b01010100},
		},
	}
	for _, test := range tests {
		bf := test.input
		bf.SetPiece(test.index)
		assert.Equal(t, bf, test.outpt)
	}
}
