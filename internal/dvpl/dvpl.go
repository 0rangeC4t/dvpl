// Package dvpl packs and unpacks DVPL files.
package dvpl

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"

	"dvpl/internal/codec"
)

const maxSize = 0x7E000000
const footerSize = 20

// Kind is the compression mode used by Pack.
type Kind = codec.Kind

const (
	None  = codec.None
	LZ4   = codec.LZ4
	LZ4HC = codec.LZ4HC
)

// mapKind handles both directions for footer type labels. These labels are not verified against real game files.
func mapKind(kind codec.Kind, label uint32, reading bool) (codec.Kind, uint32, error) {
	if reading {
		switch label {
		case 0:
			return codec.None, 0, nil
		case 1:
			return codec.LZ4, 1, nil
		case 2:
			return codec.LZ4HC, 2, nil
		case 4:
			return codec.Kind(4), 4, nil
		default:
			return 0, 0, fmt.Errorf("unsupported DVPL type %d", label)
		}
	}
	if kind == codec.Kind(4) {
		return 0, 0, errors.New("write type 4 unsupported")
	}
	switch kind {
	case codec.None:
		return codec.None, 0, nil
	case codec.LZ4:
		return codec.LZ4, 1, nil
	case codec.LZ4HC:
		return codec.LZ4HC, 2, nil
	default:
		return 0, 0, errors.New("unsupported compression kind")
	}
}

// Pack encodes data and appends a DVPL footer. forced preserves the requested
// compressed type even when compression is not smaller.
func Pack(data []byte, kind Kind, forced bool) ([]byte, error) {
	if len(data) > maxSize {
		return nil, errors.New("input exceeds DVPL size limit")
	}
	kind, label, err := mapKind(kind, 0, false)
	if err != nil {
		return nil, err
	}
	payload := data
	if kind != codec.None && len(data) > 0 {
		compressed, ok, err := codec.Compress(data, kind)
		if err != nil {
			return nil, err
		}
		if ok && (forced || len(compressed) < len(data)) {
			payload = compressed
		} else {
			label = 0
		}
	} else {
		label = 0
	}
	out := make([]byte, len(payload)+footerSize)
	copy(out, payload)
	f := out[len(payload):]
	binary.LittleEndian.PutUint32(f[0:4], uint32(len(data)))
	binary.LittleEndian.PutUint32(f[4:8], uint32(len(payload)))
	binary.LittleEndian.PutUint32(f[8:12], crc32.ChecksumIEEE(payload))
	binary.LittleEndian.PutUint32(f[12:16], label)
	copy(f[16:20], "DVPL")
	return out, nil
}

// Unpack validates the footer and returns the original data. trustCRC skips
// only the payload checksum comparison.
func Unpack(file []byte, trustCRC bool) ([]byte, error) {
	if len(file) < footerSize {
		return nil, errors.New("DVPL file shorter than footer")
	}
	f := file[len(file)-footerSize:]
	if string(f[16:20]) != "DVPL" {
		return nil, errors.New("invalid DVPL magic")
	}
	original := binary.LittleEndian.Uint32(f[0:4])
	packed := binary.LittleEndian.Uint32(f[4:8])
	checksum := binary.LittleEndian.Uint32(f[8:12])
	label := binary.LittleEndian.Uint32(f[12:16])
	payload := file[:len(file)-footerSize]
	if uint64(packed) != uint64(len(payload)) {
		return nil, errors.New("DVPL packed size mismatch")
	}
	if original > maxSize {
		return nil, errors.New("DVPL original size exceeds limit")
	}
	kind, _, err := mapKind(0, label, true)
	if err != nil {
		return nil, err
	}
	if label != 4 && !trustCRC && crc32.ChecksumIEEE(payload) != checksum {
		return nil, errors.New("DVPL CRC mismatch")
	}
	if label == 4 {
		return unpackType4(payload, original)
	}
	if kind == codec.None {
		if original != packed {
			return nil, errors.New("raw DVPL size mismatch")
		}
		return append([]byte(nil), payload...), nil
	}
	dst := make([]byte, int(original))
	n, err := codec.Decompress(payload, dst)
	if err != nil {
		return nil, fmt.Errorf("LZ4 decode: %w", err)
	}
	if n != int(original) {
		return nil, fmt.Errorf("DVPL output size mismatch: got %d, want %d", n, original)
	}
	return dst, nil
}

func unpackType4(payload []byte, original uint32) ([]byte, error) {
	if len(payload) < 28 {
		return nil, errors.New("type 4 payload shorter than header")
	}
	nblk := binary.LittleEndian.Uint32(payload[0:4])
	hsz := binary.LittleEndian.Uint32(payload[4:8])
	csz := binary.LittleEndian.Uint32(payload[16:20])
	usz := binary.LittleEndian.Uint32(payload[24:28])
	if nblk != 1 {
		return nil, fmt.Errorf("type 4 block count mismatch: got %d, want 1", nblk)
	}
	if hsz != 28 {
		return nil, fmt.Errorf("type 4 header size mismatch: got %d, want 28", hsz)
	}
	if usz != original {
		return nil, fmt.Errorf("type 4 original size mismatch: got %d, want %d", usz, original)
	}
	if uint64(hsz)+uint64(csz) != uint64(len(payload)) {
		return nil, errors.New("type 4 compressed size mismatch")
	}
	dst := make([]byte, int(usz))
	n, err := codec.Decompress(payload[hsz:hsz+csz], dst)
	if err != nil {
		return nil, fmt.Errorf("LZ4 decode: %w", err)
	}
	if n != int(usz) {
		return nil, fmt.Errorf("DVPL output size mismatch: got %d, want %d", n, usz)
	}
	return dst, nil
}
