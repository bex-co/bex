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
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// The aws-chunked framing of STREAMING-UNSIGNED-PAYLOAD-TRAILER (ADR112 D3): the
// body is a run of chunks, `<hex size>\r\n<data>\r\n`, a final `0\r\n`, the trailing
// headers (`x-amz-checksum-crc64nvme:<base64>\r\n`) and a closing `\r\n`. It sits
// inside whatever HTTP framing carried it (net/http has already undone a chunked
// transfer encoding), so the client's own digest, which it can only compute while
// it streams, arrives after the bytes.

const (
	// maxChunkLine bounds a chunk-size line and a trailer line: nothing legitimate
	// is longer, and a parser that read an unbounded line would let a client hold
	// memory with a body that never ends one.
	maxChunkLine = 4096
	// maxTrailerLines bounds the trailing headers: one is all S3 defines.
	maxTrailerLines = 8
)

// errMalformedChunks is a body that is not valid aws-chunked framing.
var errMalformedChunks = errors.New("the aws-chunked body is malformed")

// chunkedReader decodes the framing and hands out the data. Once the data ends it
// has parsed the trailer; Trailer is read after Read returned io.EOF.
type chunkedReader struct {
	br        *bufio.Reader
	remaining int64 // bytes of the current chunk not yet read
	done      bool
	err       error
	trailer   map[string]string // lowercase name -> value
}

func newChunkedReader(r io.Reader) *chunkedReader {
	return &chunkedReader{br: bufio.NewReaderSize(r, 32<<10), trailer: map[string]string{}}
}

func (c *chunkedReader) Read(p []byte) (int, error) {
	if c.err != nil {
		return 0, c.err
	}
	if c.done {
		return 0, io.EOF
	}
	if len(p) == 0 {
		return 0, nil
	}
	if c.remaining == 0 {
		if err := c.nextChunk(); err != nil {
			c.err = err
			return 0, err
		}
		if c.done {
			return 0, io.EOF
		}
	}
	if int64(len(p)) > c.remaining {
		p = p[:c.remaining]
	}
	n, err := c.br.Read(p)
	c.remaining -= int64(n)
	if err != nil && !errors.Is(err, io.EOF) {
		c.err = err
		return n, err
	}
	if errors.Is(err, io.EOF) && c.remaining > 0 {
		c.err = io.ErrUnexpectedEOF
		return n, c.err
	}
	if c.remaining == 0 {
		// The data of a chunk is followed by its own CRLF.
		if err := c.expectCRLF(); err != nil {
			c.err = err
			return n, err
		}
	}
	return n, nil
}

// nextChunk reads a chunk-size line. A chunk of size 0 is the end of the data, and
// the trailer follows.
func (c *chunkedReader) nextChunk() error {
	line, err := c.readLine()
	if err != nil {
		return err
	}
	// A signed chunk carries `;chunk-signature=`: that mode is refused before the
	// body is opened, so an extension here is a client that lied about its mode.
	if strings.Contains(line, ";") {
		return fmt.Errorf("%w: a chunk extension", errMalformedChunks)
	}
	size, err := strconv.ParseUint(line, 16, 62)
	if err != nil || line == "" {
		return fmt.Errorf("%w: chunk size %q", errMalformedChunks, truncate(line, 32))
	}
	if size == 0 {
		c.done = true
		return c.readTrailer()
	}
	c.remaining = int64(size)
	return nil
}

func (c *chunkedReader) expectCRLF() error {
	b, err := c.br.Peek(2)
	if err != nil {
		return unexpected(err)
	}
	if b[0] != '\r' || b[1] != '\n' {
		return fmt.Errorf("%w: a chunk is not followed by CRLF", errMalformedChunks)
	}
	_, _ = c.br.Discard(2)
	return nil
}

// readTrailer reads the trailing headers up to the closing blank line, and nothing
// past it.
func (c *chunkedReader) readTrailer() error {
	for i := 0; ; i++ {
		line, err := c.readLine()
		if err != nil {
			return err
		}
		if line == "" {
			return nil
		}
		if i >= maxTrailerLines {
			return fmt.Errorf("%w: too many trailing headers", errMalformedChunks)
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok || name == "" {
			return fmt.Errorf("%w: trailing header %q", errMalformedChunks, truncate(line, 32))
		}
		c.trailer[strings.ToLower(strings.TrimSpace(name))] = strings.TrimSpace(value)
	}
}

// readLine reads one CRLF-terminated line of at most maxChunkLine bytes, without
// its terminator.
func (c *chunkedReader) readLine() (string, error) {
	var line []byte
	for {
		b, err := c.br.ReadByte()
		if err != nil {
			return "", unexpected(err)
		}
		if b == '\n' {
			if len(line) == 0 || line[len(line)-1] != '\r' {
				return "", fmt.Errorf("%w: a line ends without CR", errMalformedChunks)
			}
			return string(line[:len(line)-1]), nil
		}
		line = append(line, b)
		if len(line) > maxChunkLine {
			return "", fmt.Errorf("%w: a line is longer than %d bytes", errMalformedChunks, maxChunkLine)
		}
	}
}

func unexpected(err error) error {
	if errors.Is(err, io.EOF) {
		return io.ErrUnexpectedEOF
	}
	return err
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
