package main

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/pierrec/lz4/v4"
)

const lz4ChunkCompressed = 1

func WriteLZ4Stream(w io.Writer, payload []byte, blockSize int) error {
	if blockSize <= 0 {
		blockSize = lz4BlockSize
	}
	for len(payload) > 0 {
		n := min(blockSize, len(payload))
		block := payload[:n]
		payload = payload[n:]

		compressed := make([]byte, lz4.CompressBlockBound(len(block)))
		compressedLen, err := lz4.CompressBlock(block, compressed, nil)
		if err != nil {
			return err
		}
		if compressedLen > 0 && compressedLen < len(block) {
			if err := writeUvarint(w, lz4ChunkCompressed); err != nil {
				return err
			}
			if err := writeUvarint(w, uint64(len(block))); err != nil {
				return err
			}
			if err := writeUvarint(w, uint64(compressedLen)); err != nil {
				return err
			}
			if _, err := w.Write(compressed[:compressedLen]); err != nil {
				return err
			}
			continue
		}

		if err := writeUvarint(w, 0); err != nil {
			return err
		}
		if err := writeUvarint(w, uint64(len(block))); err != nil {
			return err
		}
		if _, err := w.Write(block); err != nil {
			return err
		}
	}
	return nil
}

func ReadLZ4Stream(r io.Reader) ([]byte, error) {
	var out []byte
	for {
		flags, err := readUvarint(r)
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}

		rawLen, err := readUvarint(r)
		if err != nil {
			return nil, err
		}
		storedLen := rawLen
		if flags&lz4ChunkCompressed != 0 {
			storedLen, err = readUvarint(r)
			if err != nil {
				return nil, err
			}
		}
		if storedLen > rawLen && flags&lz4ChunkCompressed != 0 {
			return nil, fmt.Errorf("compressed chunk length %d exceeds raw length %d", storedLen, rawLen)
		}

		block := make([]byte, int(storedLen))
		if _, err := io.ReadFull(r, block); err != nil {
			return nil, err
		}

		if flags&lz4ChunkCompressed == 0 {
			out = append(out, block...)
			continue
		}
		if flags>>2 != 0 {
			return nil, fmt.Errorf("unsupported multi-pass LZ4 chunk flags %d", flags)
		}

		decoded, err := decodeLZ4Block(block, int(rawLen))
		if err != nil {
			return nil, err
		}
		out = append(out, decoded...)
	}
}

func writeUvarint(w io.Writer, value uint64) error {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], value)
	_, err := w.Write(buf[:n])
	return err
}

func readUvarint(r io.Reader) (uint64, error) {
	var result uint64
	var shift uint
	var one [1]byte
	for i := 0; i < binary.MaxVarintLen64; i++ {
		n, err := r.Read(one[:])
		if err != nil {
			return 0, err
		}
		if n != 1 {
			return 0, io.ErrUnexpectedEOF
		}
		b := one[0]
		result |= uint64(b&0x7f) << shift
		if b < 0x80 {
			return result, nil
		}
		shift += 7
	}
	return 0, fmt.Errorf("varint too long")
}

func decodeLZ4Block(src []byte, rawLen int) ([]byte, error) {
	dst := make([]byte, 0, rawLen)
	for i := 0; i < len(src); {
		token := src[i]
		i++

		litLen := int(token >> 4)
		if litLen == 15 {
			extra, next, err := readLZ4Length(src, i)
			if err != nil {
				return nil, err
			}
			litLen += extra
			i = next
		}
		if i+litLen > len(src) {
			return nil, fmt.Errorf("truncated LZ4 literals")
		}
		dst = append(dst, src[i:i+litLen]...)
		i += litLen

		if i >= len(src) {
			break
		}
		if i+2 > len(src) {
			return nil, fmt.Errorf("truncated LZ4 offset")
		}
		offset := int(src[i]) | int(src[i+1])<<8
		i += 2
		if offset <= 0 || offset > len(dst) {
			return nil, fmt.Errorf("invalid LZ4 offset %d", offset)
		}

		matchLen := int(token&0x0f) + 4
		if token&0x0f == 15 {
			extra, next, err := readLZ4Length(src, i)
			if err != nil {
				return nil, err
			}
			matchLen += extra
			i = next
		}
		for j := 0; j < matchLen; j++ {
			dst = append(dst, dst[len(dst)-offset])
		}
		if len(dst) > rawLen {
			return nil, fmt.Errorf("LZ4 decoded past expected length")
		}
	}
	if len(dst) != rawLen {
		return nil, fmt.Errorf("LZ4 decoded %d bytes, expected %d", len(dst), rawLen)
	}
	return dst, nil
}

func readLZ4Length(src []byte, i int) (int, int, error) {
	total := 0
	for {
		if i >= len(src) {
			return 0, 0, fmt.Errorf("truncated LZ4 length")
		}
		b := int(src[i])
		i++
		total += b
		if b != 255 {
			return total, i, nil
		}
	}
}
