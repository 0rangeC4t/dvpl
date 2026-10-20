package dvpl

import (
	"encoding/binary"
	"math/rand"
	"testing"
)

func footer(f []byte) []byte { return f[len(f)-footerSize:] }

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		kind Kind
		data []byte
	}{
		{"none", None, []byte("raw payload")},
		{"lz4", LZ4, bytesRepeat("DVPL block data ", 800)},
		{"lz4hc", LZ4HC, bytesRepeat("DVPL block data ", 800)},
		{"empty", LZ4, nil},
		{"one byte", LZ4HC, []byte{7}},
		{"twenty bytes", LZ4, []byte("12345678901234567890")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Pack(tt.data, tt.kind, false)
			if err != nil {
				t.Fatal(err)
			}
			out, err := Unpack(got, false)
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != string(tt.data) {
				t.Fatalf("round trip mismatch")
			}
		})
	}
}

func TestRandomFallsBackAndRepeatedCompresses(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	random := make([]byte, 4096)
	_, _ = rng.Read(random)
	for _, data := range [][]byte{random, bytesRepeat("compressible", 4096)} {
		file, err := Pack(data, LZ4HC, false)
		if err != nil {
			t.Fatal(err)
		}
		label := binary.LittleEndian.Uint32(footer(file)[12:16])
		if data == nil {
			t.Fatal("unreachable")
		}
		if len(file)-footerSize >= len(data) && label != 0 {
			t.Fatalf("expected raw type, got %d", label)
		}
		if len(file)-footerSize < len(data) && label == 0 {
			t.Fatal("expected compressed type")
		}
	}
}

func TestMalformedFooters(t *testing.T) {
	valid, err := Pack(bytesRepeat("repeated", 100), LZ4, false)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func([]byte)
	}{
		{"magic", func(b []byte) { footer(b)[16] = 'X' }},
		{"crc", func(b []byte) { footer(b)[8] ^= 1 }},
		{"packed size", func(b []byte) { binary.LittleEndian.PutUint32(footer(b)[4:8], 99) }},
		{"original size", func(b []byte) { binary.LittleEndian.PutUint32(footer(b)[0:4], 99) }},
		{"unknown type", func(b []byte) { binary.LittleEndian.PutUint32(footer(b)[12:16], 3) }},
		{"oversize original", func(b []byte) { binary.LittleEndian.PutUint32(footer(b)[0:4], maxSize+1) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := append([]byte(nil), valid...)
			tt.mutate(b)
			if _, err := Unpack(b, false); err == nil {
				t.Fatal("expected error")
			}
		})
	}
	if _, err := Unpack([]byte{1, 2, 3}, false); err == nil {
		t.Fatal("expected short file error")
	}
}

func bytesRepeat(s string, n int) []byte {
	b := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		b = append(b, s...)
	}
	return b
}
