/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package s3gw

import (
	"crypto/md5"  //nolint:gosec // Content-MD5 is S3's integrity check, not a security use
	"crypto/sha1" //nolint:gosec // x-amz-checksum-sha1 is S3's integrity check, not a security use
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"hash"
	"hash/crc32"
	"hash/crc64"
	"strings"
	"sync"
)

// The additional checksums S3 clients send (ADR112 D3): CRC32, CRC32C, CRC64NVME,
// SHA1 and SHA256, as the x-amz-checksum-<algorithm> header or as a trailer, and
// Content-MD5. bex verifies each before it commits the object and echoes the ones
// it verified. Any other x-amz-checksum-* answers 501, so integrity is never
// skipped silently.

// checksumAlgorithms are the algorithms by the name in x-amz-checksum-<name>.
var checksumAlgorithms = map[string]func() hash.Hash{
	"crc32":     func() hash.Hash { return crc32.NewIEEE() },
	"crc32c":    func() hash.Hash { return crc32.New(crc32.MakeTable(crc32.Castagnoli)) },
	"crc64nvme": func() hash.Hash { return newCRC64NVME() },
	"sha1":      func() hash.Hash { return sha1.New() }, //nolint:gosec
	"sha256":    func() hash.Hash { return sha256.New() },
}

// CRC-64/NVME: width 64, polynomial 0xAD93D23594C93659 (0x9A6C9329AC4BC9B5 bit-reversed, the
// form hash/crc64 takes), initial value and final XOR all ones, input and output reflected.
// Its check value for "123456789" is 0xAE8B14860A799888. The AWS CLI v2 sends it by default.
// hash/crc64 serves only its ECMA and ISO tables fast: for any other table it rebuilds a
// slicing-by-8 table on every write of 2 KiB or more, so the digest is computed here, with
// the table built once.
const crc64NVMEPoly = 0x9A6C9329AC4BC9B5

var (
	crc64NVMEOnce  sync.Once
	crc64NVMETable [8][256]uint64
)

func buildCRC64NVMETable() {
	t := crc64.MakeTable(crc64NVMEPoly)
	crc64NVMETable[0] = *t
	for i := range 256 {
		crc := t[i]
		for j := 1; j < 8; j++ {
			crc = t[crc&0xff] ^ (crc >> 8)
			crc64NVMETable[j][i] = crc
		}
	}
}

// crc64NVME is a hash.Hash64 over CRC-64/NVME.
type crc64NVME struct{ crc uint64 }

func newCRC64NVME() *crc64NVME {
	crc64NVMEOnce.Do(buildCRC64NVMETable)
	return &crc64NVME{}
}

func (c *crc64NVME) Write(p []byte) (int, error) {
	n := len(p)
	crc := ^c.crc
	for len(p) >= 8 {
		crc ^= uint64(p[0]) | uint64(p[1])<<8 | uint64(p[2])<<16 | uint64(p[3])<<24 |
			uint64(p[4])<<32 | uint64(p[5])<<40 | uint64(p[6])<<48 | uint64(p[7])<<56
		crc = crc64NVMETable[7][crc&0xff] ^ crc64NVMETable[6][(crc>>8)&0xff] ^ crc64NVMETable[5][(crc>>16)&0xff] ^
			crc64NVMETable[4][(crc>>24)&0xff] ^ crc64NVMETable[3][(crc>>32)&0xff] ^ crc64NVMETable[2][(crc>>40)&0xff] ^
			crc64NVMETable[1][(crc>>48)&0xff] ^ crc64NVMETable[0][crc>>56]
		p = p[8:]
	}
	for _, v := range p {
		crc = crc64NVMETable[0][byte(crc)^v] ^ (crc >> 8)
	}
	c.crc = ^crc
	return n, nil
}

func (c *crc64NVME) Sum64() uint64 { return c.crc }
func (c *crc64NVME) Sum(b []byte) []byte {
	return binary.BigEndian.AppendUint64(b, c.crc)
}
func (c *crc64NVME) Reset()         { c.crc = 0 }
func (c *crc64NVME) Size() int      { return 8 }
func (c *crc64NVME) BlockSize() int { return 1 }

const checksumHeaderPrefix = "x-amz-checksum-"

// checksumName is the algorithm a header or trailer name carries, ok false when
// the name is not an x-amz-checksum-* at all.
func checksumName(header string) (algorithm string, ok bool) {
	lower := strings.ToLower(header)
	if !strings.HasPrefix(lower, checksumHeaderPrefix) {
		return "", false
	}
	return strings.TrimPrefix(lower, checksumHeaderPrefix), true
}

// encodeChecksum is the base64 of a digest as S3 shows it: the big-endian bytes of
// a CRC, the digest of a hash.
func encodeChecksum(h hash.Hash) string {
	switch c := h.(type) {
	case hash.Hash32:
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], c.Sum32())
		return base64.StdEncoding.EncodeToString(b[:])
	case hash.Hash64:
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], c.Sum64())
		return base64.StdEncoding.EncodeToString(b[:])
	}
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// md5Hash is the digest of Content-MD5.
func md5Hash() hash.Hash { return md5.New() } //nolint:gosec
