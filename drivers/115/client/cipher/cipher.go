// Package cipher115 implements the reference fixed-key wire transforms.
// See NOTICE and LICENSE.upstream for provenance and unresolved license metadata.
package cipher115

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math/big"
)

var aesKey = []byte{0xfb, 0x1a, 0x19, 0xd6, 0x52, 0xf5, 0xaa, 0xf7, 0xbc, 0x65, 0x1d, 0x0f, 0x69, 0xbf, 0x42, 0x2f}
var aesIV = []byte{0x69, 0xbf, 0x42, 0x2f, 0x49, 0x96, 0x05, 0x50, 0xa0, 0xad, 0x44, 0xec, 0x34, 0x46, 0xcb, 0x4c}

func xor(data, key []byte) []byte {
	out := make([]byte, len(data))
	n := len(data) & 3
	for i := 0; i < n; i++ {
		out[i] = data[i] ^ key[i]
	}
	for i := n; i < len(data); i++ {
		out[i] = data[i] ^ key[(i-n)%len(key)]
	}
	return out
}
func reverse(b []byte) {
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
}
func transform(b []byte) []byte {
	n, _ := new(big.Int).SetString(modulus, 16)
	v := new(big.Int).Exp(new(big.Int).SetBytes(b), big.NewInt(65537), n)
	return v.Bytes()
}
func RSAEncrypt(data []byte) string {
	b := xor(data, []byte{0x8d, 0xa5, 0xa5, 0x8d})
	reverse(b)
	b = append(make([]byte, 16), xor(b, []byte("x\x06\xadL3\x86]\x18L\x01?F"))...)
	var out []byte
	for len(b) > 0 {
		n := len(b)
		if n > 117 {
			n = 117
		}
		block := append([]byte{0}, bytes.Repeat([]byte{2}, 126-n)...)
		block = append(block, 0)
		block = append(block, b[:n]...)
		v := transform(block)
		out = append(out, make([]byte, 128-len(v))...)
		out = append(out, v...)
		b = b[n:]
	}
	return base64.StdEncoding.EncodeToString(out)
}
func RSADecrypt(s string) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(b)%128 != 0 {
		return nil, fmt.Errorf("invalid RSA block length")
	}
	var out []byte
	for len(b) > 0 {
		v := transform(b[:128])
		i := bytes.IndexByte(v, 0)
		if i < 0 {
			return nil, fmt.Errorf("invalid RSA padding")
		}
		out = append(out, v[i+1:]...)
		b = b[128:]
	}
	if len(out) < 16 {
		return nil, fmt.Errorf("short RSA payload")
	}
	key := make([]byte, 12)
	for i := range key {
		key[i] = keyTable[132-i*12] ^ byte(int(out[i])+int(keyTable[i*12]))
	}
	out = xor(out[16:], key)
	reverse(out)
	return xor(out, []byte{0x8d, 0xa5, 0xa5, 0x8d}), nil
}
func AESEncrypt(data []byte) []byte {
	b := append([]byte(nil), data...)
	if n := (-len(b)) & 15; n != 0 {
		b = append(b, bytes.Repeat([]byte{byte(n)}, n)...)
	}
	block, _ := aes.NewCipher(aesKey)
	cipher.NewCBCEncrypter(block, aesIV).CryptBlocks(b, b)
	return b
}
func Token() string {
	pub := []byte("\x1d\x03\x0e\x80\xa1x\xdc\xee\xce\xcd\xa3w\xde\x12\x8d\x8e\xd9\xdd\xcfU\xaea\xedF\xea\x12\x1a\x1c\xfc\x81")
	b := append([]byte(nil), pub[:15]...)
	b = append(b, 0, 0x73, 0, 0, 0, 0, 0, 0, 0)
	b = append(b, pub[15:]...)
	b = append(b, 0, 1, 0, 0, 0)
	sum := crc32.ChecksumIEEE(append([]byte("^j>WD3Kr?J2gLFjD4W2y@"), b...))
	b = binary.LittleEndian.AppendUint32(b, sum)
	return base64.StdEncoding.EncodeToString(b)
}
func AESDecrypt(data []byte) ([]byte, error) {
	n := len(data) & -16
	if n == 0 {
		return nil, fmt.Errorf("short AES payload")
	}
	b := append([]byte(nil), data[:n]...)
	block, _ := aes.NewCipher(aesKey)
	cipher.NewCBCDecrypter(block, aesIV).CryptBlocks(b, b)
	pad := int(b[len(b)-1])
	if pad > 0 && pad < 16 && pad <= len(b) && bytes.Equal(b[len(b)-pad:], bytes.Repeat([]byte{byte(pad)}, pad)) {
		b = b[:len(b)-pad]
	}
	var out []byte
	for len(b) > 2 {
		n := int(binary.LittleEndian.Uint16(b))
		if n == 0 {
			break
		}
		if n > len(b)-2 {
			return nil, fmt.Errorf("short LZ4 frame")
		}
		v, err := lz4(b[2 : 2+n])
		if err != nil {
			return nil, err
		}
		out = append(out, v...)
		b = b[2+n:]
	}
	return out, nil
}
func lz4(src []byte) ([]byte, error) {
	out := make([]byte, 0, 8192)
	readLen := func(n int) (int, error) {
		if n == 15 {
			for {
				if len(src) == 0 {
					return 0, fmt.Errorf("short LZ4 length")
				}
				v := int(src[0])
				src = src[1:]
				n += v
				if v != 255 {
					break
				}
			}
		}
		return n, nil
	}
	for len(src) > 0 {
		token := src[0]
		src = src[1:]
		n, err := readLen(int(token >> 4))
		if err != nil {
			return nil, err
		}
		if n > len(src) || len(out)+n > 8192 {
			return nil, fmt.Errorf("invalid LZ4 literals")
		}
		out = append(out, src[:n]...)
		src = src[n:]
		if len(src) == 0 {
			break
		}
		if len(src) < 2 {
			return nil, fmt.Errorf("short LZ4 offset")
		}
		offset := int(binary.LittleEndian.Uint16(src))
		src = src[2:]
		n, err = readLen(int(token & 15))
		if err != nil {
			return nil, err
		}
		n += 4
		if offset == 0 || offset > len(out) || len(out)+n > 8192 {
			return nil, fmt.Errorf("invalid LZ4 match")
		}
		for i := 0; i < n; i++ {
			out = append(out, out[len(out)-offset])
		}
	}
	return out, nil
}
