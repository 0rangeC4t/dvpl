//go:build !cgo || purego

package codec

import (
	"errors"

	"github.com/pierrec/lz4/v4"
)

func compress(src []byte, kind Kind) ([]byte, bool, error) {
	if kind != LZ4 && kind != LZ4HC {
		return nil, false, errors.New("unsupported compression kind")
	}
	if len(src) == 0 {
		return nil, false, nil
	}
	dst := make([]byte, lz4.CompressBlockBound(len(src)))
	var n int
	var err error
	if kind == LZ4HC {
		n, err = lz4.CompressBlockHC(src, dst, lz4.Level9, nil, nil)
	} else {
		n, err = lz4.CompressBlock(src, dst, nil)
	}
	if err != nil {
		return nil, false, err
	}
	if n == 0 {
		return nil, false, nil
	}
	return dst[:n], true, nil
}

func decompress(src, dst []byte) (int, error) { return lz4.UncompressBlock(src, dst) }
