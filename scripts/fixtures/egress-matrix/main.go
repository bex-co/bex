// Copyright 2026.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// egress-fixture provides dependency-free protocol peers for the isolated
// outbound-bandwidth traffic matrix. It is not shipped in a bex image.
package main

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // WebSocket's wire protocol requires SHA-1.
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	websocketKey  = "dGhlIHNhbXBsZSBub25jZQ=="
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: egress-fixture ws-server|ws-client|tls-server|tls-client|pg-server|pg-client ...")
	}
	var err error
	switch os.Args[1] {
	case "ws-server":
		addr := ":8080"
		if len(os.Args) > 2 {
			addr = os.Args[2]
		}
		err = serveWebSocket(addr)
	case "ws-client":
		err = runWebSocketClient(os.Args[2:])
	case "tls-server":
		err = serveTLSFixture(argument(os.Args[2:], 0, ":6380"), false)
	case "tls-client":
		err = runTLSFixtureClient(os.Args[2:], false)
	case "pg-server":
		err = serveTLSFixture(argument(os.Args[2:], 0, ":5432"), true)
	case "pg-client":
		err = runTLSFixtureClient(os.Args[2:], true)
	case "idle":
		for {
			time.Sleep(time.Hour)
		}
	case "public-server":
		err = servePublicFixture(os.Args[2:])
	case "direct-client":
		err = runDirectClient(os.Args[2:])
	default:
		err = fmt.Errorf("unknown mode %q", os.Args[1])
	}
	if err != nil {
		log.Fatal(err)
	}
}

func argument(args []string, index int, fallback string) string {
	if len(args) > index {
		return args[index]
	}
	return fallback
}

func serveWebSocket(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/bytes", func(writer http.ResponseWriter, request *http.Request) {
		count, err := strconv.Atoi(request.URL.Query().Get("n"))
		if err != nil || count < 0 || count > 1<<20 {
			http.Error(writer, "invalid n", http.StatusBadRequest)
			return
		}
		writer.Header().Set("Content-Type", "application/octet-stream")
		_, _ = writer.Write(bytes.Repeat([]byte{'h'}, count))
	})
	mux.HandleFunc("/ws", func(writer http.ResponseWriter, request *http.Request) {
		if err := websocketServer(writer, request); err != nil {
			log.Printf("websocket: %v", err)
		}
	})
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	return server.ListenAndServe()
}

func websocketServer(writer http.ResponseWriter, request *http.Request) error {
	if !strings.EqualFold(request.Header.Get("Upgrade"), "websocket") {
		http.Error(writer, "websocket upgrade required", http.StatusUpgradeRequired)
		return nil
	}
	key := request.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		http.Error(writer, "missing websocket key", http.StatusBadRequest)
		return nil
	}
	hijacker, ok := writer.(http.Hijacker)
	if !ok {
		return errors.New("HTTP server does not support hijacking")
	}
	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return err
	}
	defer conn.Close()
	hash := sha1.Sum([]byte(key + websocketGUID)) //nolint:gosec // WebSocket protocol requirement.
	if _, err := fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", base64.StdEncoding.EncodeToString(hash[:])); err != nil {
		return err
	}
	if err := rw.Flush(); err != nil {
		return err
	}
	requestPayload, _, err := readFrame(rw.Reader)
	if err != nil {
		return fmt.Errorf("read request frame: %w", err)
	}
	responseBytes, err := strconv.Atoi(request.Header.Get("X-Response-Bytes"))
	if err != nil || responseBytes < 1 || responseBytes > 1<<20 {
		return fmt.Errorf("invalid X-Response-Bytes %q", request.Header.Get("X-Response-Bytes"))
	}
	if _, err := writeFrame(rw.Writer, bytes.Repeat([]byte{'s'}, responseBytes), false); err != nil {
		return err
	}
	if err := rw.Flush(); err != nil {
		return err
	}
	log.Printf("served request_payload=%d response_payload=%d", len(requestPayload), responseBytes)
	return nil
}

func runWebSocketClient(args []string) error {
	if len(args) < 2 {
		return errors.New("usage: egress-fixture ws-client <addr> <host> [request-bytes] [response-bytes]")
	}
	requestBytes, responseBytes := 2048, 4096
	var err error
	if len(args) > 2 {
		requestBytes, err = strconv.Atoi(args[2])
		if err != nil {
			return err
		}
	}
	if len(args) > 3 {
		responseBytes, err = strconv.Atoi(args[3])
		if err != nil {
			return err
		}
	}
	conn, err := net.DialTimeout("tcp", args[0], 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := fmt.Fprintf(conn, "GET /ws HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\nX-Response-Bytes: %d\r\n\r\n", args[1], websocketKey, responseBytes); err != nil {
		return err
	}
	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodGet})
	if err != nil {
		return fmt.Errorf("read upgrade response: %w", err)
	}
	if response.StatusCode != http.StatusSwitchingProtocols {
		return fmt.Errorf("upgrade status: %s", response.Status)
	}
	if response.Header.Get("Sec-WebSocket-Accept") != expectedAccept(websocketKey) {
		return errors.New("invalid Sec-WebSocket-Accept")
	}
	requestWireBytes, err := writeFrame(conn, bytes.Repeat([]byte{'c'}, requestBytes), true)
	if err != nil {
		return err
	}
	payload, responseWireBytes, err := readFrame(reader)
	if err != nil {
		return err
	}
	if len(payload) != responseBytes {
		return fmt.Errorf("response payload: got %d, want %d", len(payload), responseBytes)
	}
	fmt.Printf("request_payload=%d request_wire=%d response_payload=%d response_wire=%d\n", requestBytes, requestWireBytes, responseBytes, responseWireBytes)
	return nil
}

func expectedAccept(key string) string {
	hash := sha1.Sum([]byte(key + websocketGUID)) //nolint:gosec // WebSocket protocol requirement.
	return base64.StdEncoding.EncodeToString(hash[:])
}

func writeFrame(writer io.Writer, payload []byte, masked bool) (int, error) {
	header := []byte{0x82}
	maskBit := byte(0)
	if masked {
		maskBit = 0x80
	}
	switch {
	case len(payload) < 126:
		header = append(header, maskBit|byte(len(payload)))
	case len(payload) <= 65535:
		header = append(header, maskBit|126, 0, 0)
		binary.BigEndian.PutUint16(header[len(header)-2:], uint16(len(payload)))
	default:
		header = append(header, maskBit|127, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(header[len(header)-8:], uint64(len(payload)))
	}
	body := append([]byte(nil), payload...)
	if masked {
		mask := [4]byte{0x13, 0x37, 0xc0, 0xde}
		header = append(header, mask[:]...)
		for i := range body {
			body[i] ^= mask[i%len(mask)]
		}
	}
	n, err := writer.Write(append(header, body...))
	return n, err
}

func readFrame(reader io.Reader) ([]byte, int, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, 0, err
	}
	length := uint64(header[1] & 0x7f)
	extendedBytes := 0
	switch length {
	case 126:
		encoded := make([]byte, 2)
		if _, err := io.ReadFull(reader, encoded); err != nil {
			return nil, 0, err
		}
		length = uint64(binary.BigEndian.Uint16(encoded))
		extendedBytes = 2
	case 127:
		encoded := make([]byte, 8)
		if _, err := io.ReadFull(reader, encoded); err != nil {
			return nil, 0, err
		}
		length = binary.BigEndian.Uint64(encoded)
		extendedBytes = 8
	}
	if length > 1<<20 {
		return nil, 0, fmt.Errorf("frame length %d exceeds fixture limit", length)
	}
	maskBytes := 0
	var mask [4]byte
	if header[1]&0x80 != 0 {
		if _, err := io.ReadFull(reader, mask[:]); err != nil {
			return nil, 0, err
		}
		maskBytes = len(mask)
	}
	payload := make([]byte, int(length))
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, 0, err
	}
	if maskBytes > 0 {
		for i := range payload {
			payload[i] ^= mask[i%len(mask)]
		}
	}
	return payload, 2 + extendedBytes + maskBytes + len(payload), nil
}

func serveTLSFixture(addr string, postgres bool) error {
	certificate, err := selfSignedCertificate()
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	for {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return acceptErr
		}
		go func() {
			if err := handleTLSFixture(conn, certificate, postgres); err != nil {
				log.Printf("TLS fixture: %v", err)
			}
		}()
	}
}

func handleTLSFixture(conn net.Conn, certificate tls.Certificate, postgres bool) error {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	if postgres {
		var preamble [8]byte
		if _, err := io.ReadFull(conn, preamble[:]); err != nil {
			return err
		}
		if binary.BigEndian.Uint32(preamble[0:4]) != 8 || binary.BigEndian.Uint32(preamble[4:8]) != 80877103 {
			return errors.New("invalid PostgreSQL SSLRequest")
		}
		if _, err := conn.Write([]byte{'S'}); err != nil {
			return err
		}
	}
	tlsConn := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS13})
	if err := tlsConn.Handshake(); err != nil {
		return err
	}
	var lengths [8]byte
	if _, err := io.ReadFull(tlsConn, lengths[:]); err != nil {
		return err
	}
	requestBytes := binary.BigEndian.Uint32(lengths[0:4])
	responseBytes := binary.BigEndian.Uint32(lengths[4:8])
	if requestBytes > 1<<20 || responseBytes > 1<<20 {
		return errors.New("fixture payload exceeds 1 MiB")
	}
	if _, err := io.CopyN(io.Discard, tlsConn, int64(requestBytes)); err != nil {
		return err
	}
	if _, err := tlsConn.Write(bytes.Repeat([]byte{'d'}, int(responseBytes))); err != nil {
		return err
	}
	return tlsConn.CloseWrite()
}

func runTLSFixtureClient(args []string, postgres bool) error {
	if len(args) < 2 {
		return errors.New("usage: egress-fixture tls-client|pg-client <addr> <sni> [request-bytes] [response-bytes]")
	}
	requestBytes, responseBytes := 65536, 4096
	var err error
	if len(args) > 2 {
		requestBytes, err = strconv.Atoi(args[2])
		if err != nil {
			return err
		}
	}
	if len(args) > 3 {
		responseBytes, err = strconv.Atoi(args[3])
		if err != nil {
			return err
		}
	}
	conn, err := net.DialTimeout("tcp", args[0], 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	if postgres {
		var preamble [8]byte
		binary.BigEndian.PutUint32(preamble[0:4], 8)
		binary.BigEndian.PutUint32(preamble[4:8], 80877103)
		if _, err := conn.Write(preamble[:]); err != nil {
			return err
		}
		var response [1]byte
		if _, err := io.ReadFull(conn, response[:]); err != nil {
			return err
		}
		if response[0] != 'S' {
			return fmt.Errorf("PostgreSQL proxy declined TLS: %q", response[0])
		}
	}
	tlsConn := tls.Client(conn, &tls.Config{
		ServerName: args[1], MinVersion: tls.VersionTLS13,
		InsecureSkipVerify: true, //nolint:gosec // Ephemeral self-signed live fixture.
	})
	if err := tlsConn.Handshake(); err != nil {
		return err
	}
	var lengths [8]byte
	binary.BigEndian.PutUint32(lengths[0:4], uint32(requestBytes))
	binary.BigEndian.PutUint32(lengths[4:8], uint32(responseBytes))
	if _, err := tlsConn.Write(append(lengths[:], bytes.Repeat([]byte{'c'}, requestBytes)...)); err != nil {
		return err
	}
	if _, err := io.CopyN(io.Discard, tlsConn, int64(responseBytes)); err != nil {
		return err
	}
	fmt.Printf("request_payload=%d response_payload=%d tls=1 postgres_preamble=%t\n", requestBytes, responseBytes, postgres)
	return nil
}

func selfSignedCertificate() (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		return tls.Certificate{}, err
	}
	template := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "egress-matrix.invalid"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"*.db.bex.co", "*.kv.bex.co"},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, nil
}

func servePublicFixture(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: egress-fixture public-server <addr>")
	}
	tcpListener, err := net.Listen("tcp", args[0])
	if err != nil {
		return err
	}
	udpAddr, err := net.ResolveUDPAddr("udp", args[0])
	if err != nil {
		return err
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	go func() {
		buffer := make([]byte, 65535)
		for {
			n, peer, readErr := udpConn.ReadFromUDP(buffer)
			if readErr != nil {
				return
			}
			if n >= 4 {
				responseBytes := int(binary.BigEndian.Uint32(buffer[:4]))
				if responseBytes <= 65507 {
					_, _ = udpConn.WriteToUDP(bytes.Repeat([]byte{'r'}, responseBytes), peer)
				}
			}
		}
	}()
	for {
		conn, acceptErr := tcpListener.Accept()
		if acceptErr != nil {
			return acceptErr
		}
		go func() {
			defer conn.Close()
			var lengths [8]byte
			if _, err := io.ReadFull(conn, lengths[:]); err != nil {
				return
			}
			requestBytes := binary.BigEndian.Uint32(lengths[0:4])
			responseBytes := binary.BigEndian.Uint32(lengths[4:8])
			if requestBytes > 1<<20 || responseBytes > 1<<20 {
				return
			}
			if _, err := io.CopyN(io.Discard, conn, int64(requestBytes)); err != nil {
				return
			}
			_, _ = conn.Write(bytes.Repeat([]byte{'r'}, int(responseBytes)))
		}()
	}
}

func runDirectClient(args []string) error {
	if len(args) < 2 {
		return errors.New("usage: egress-fixture direct-client <tcp|udp> <addr> [request-bytes] [response-bytes]")
	}
	requestBytes, responseBytes := 4096, 1024
	var err error
	if len(args) > 2 {
		requestBytes, err = strconv.Atoi(args[2])
		if err != nil {
			return err
		}
	}
	if len(args) > 3 {
		responseBytes, err = strconv.Atoi(args[3])
		if err != nil {
			return err
		}
	}
	if args[0] == "udp" && requestBytes+4 > 65507 {
		return errors.New("UDP request exceeds one datagram")
	}
	conn, err := net.DialTimeout(args[0], args[1], 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	var lengths [8]byte
	binary.BigEndian.PutUint32(lengths[0:4], uint32(requestBytes))
	binary.BigEndian.PutUint32(lengths[4:8], uint32(responseBytes))
	if args[0] == "udp" {
		payload := append(lengths[4:8], bytes.Repeat([]byte{'q'}, requestBytes)...)
		if _, err := conn.Write(payload); err != nil {
			return err
		}
	} else if _, err := conn.Write(append(lengths[:], bytes.Repeat([]byte{'q'}, requestBytes)...)); err != nil {
		return err
	}
	if _, err := io.CopyN(io.Discard, conn, int64(responseBytes)); err != nil {
		return err
	}
	fmt.Printf("protocol=%s request_payload=%d response_payload=%d\n", args[0], requestBytes, responseBytes)
	return nil
}
