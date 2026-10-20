// Package codec provides the block compression operations used by DVPL.
package codec

// Kind selects the compression effort for an LZ4 block.
type Kind uint8

const (
	None Kind = iota
	LZ4
	LZ4HC
)

// Compress compresses src with the selected LZ4 mode. A false compressed
// result means the block codec could not produce a smaller block.
func Compress(src []byte, kind Kind) ([]byte, bool, error) { return compress(src, kind) }

// Decompress decodes src into the caller-sized dst buffer.
func Decompress(src, dst []byte) (int, error) { return decompress(src, dst) }
