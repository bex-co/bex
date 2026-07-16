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

package sniproxy

import (
	"fmt"
	"io"
	"net"
	"net/netip"
)

func RemoteIP(addr net.Addr) (netip.Addr, error) {
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return netip.Addr{}, err
	}
	return netip.ParseAddr(host)
}

const (
	TLSHandshakeType = byte(0x16)
	MaxClientHello   = 16 * 1024
)

// ReadTLSRecord reads one bounded TLS record and prepends bytes already
// consumed by a protocol preamble detector.
func ReadTLSRecord(r io.Reader, initial []byte) ([]byte, error) {
	header := make([]byte, 5)
	n := copy(header, initial)
	if n < len(header) {
		if _, err := io.ReadFull(r, header[n:]); err != nil {
			return nil, fmt.Errorf("read TLS header: %w", err)
		}
	}
	if header[0] != TLSHandshakeType {
		return nil, fmt.Errorf("not a TLS handshake record: byte0=0x%02x", header[0])
	}
	recordLen := int(header[3])<<8 | int(header[4])
	if recordLen > MaxClientHello {
		return nil, fmt.Errorf("TLS record too large: %d", recordLen)
	}
	record := make([]byte, len(header)+recordLen)
	copy(record, header)
	bodyFromInitial := initial[min(len(header), len(initial)):]
	if len(bodyFromInitial) > recordLen {
		bodyFromInitial = bodyFromInitial[:recordLen]
	}
	copy(record[len(header):], bodyFromInitial)
	alreadyHave := len(header) + len(bodyFromInitial)
	if alreadyHave < len(record) {
		if _, err := io.ReadFull(r, record[alreadyHave:]); err != nil {
			return nil, fmt.Errorf("read TLS body: %w", err)
		}
	}
	return record, nil
}

// ExtractSNI parses the first host_name from a TLS ClientHello record.
func ExtractSNI(record []byte) (string, error) {
	if len(record) < 5 || record[0] != TLSHandshakeType {
		return "", fmt.Errorf("not a handshake record")
	}
	recordLen := int(record[3])<<8 | int(record[4])
	if len(record) < 5+recordLen {
		return "", fmt.Errorf("record truncated")
	}
	data := record[5 : 5+recordLen]
	if len(data) < 4 || data[0] != 0x01 {
		return "", fmt.Errorf("not a ClientHello")
	}
	handshakeLen := int(data[1])<<16 | int(data[2])<<8 | int(data[3])
	if len(data) < 4+handshakeLen {
		return "", fmt.Errorf("ClientHello truncated")
	}
	clientHello := data[4 : 4+handshakeLen]
	if len(clientHello) < 34 {
		return "", fmt.Errorf("ClientHello too short")
	}
	position := 34
	if position >= len(clientHello) {
		return "", nil
	}
	position += 1 + int(clientHello[position])
	if position+2 > len(clientHello) {
		return "", nil
	}
	position += 2 + (int(clientHello[position])<<8 | int(clientHello[position+1]))
	if position >= len(clientHello) {
		return "", nil
	}
	position += 1 + int(clientHello[position])
	if position+2 > len(clientHello) {
		return "", nil
	}
	extensionsLen := int(clientHello[position])<<8 | int(clientHello[position+1])
	position += 2
	end := min(position+extensionsLen, len(clientHello))
	for position+4 <= end {
		extensionType := uint16(clientHello[position])<<8 | uint16(clientHello[position+1])
		extensionLen := int(clientHello[position+2])<<8 | int(clientHello[position+3])
		position += 4
		if position+extensionLen > end {
			return "", fmt.Errorf("TLS extension truncated")
		}
		if extensionType == 0 {
			if name, ok := parseSNIExtension(clientHello[position : position+extensionLen]); ok {
				return name, nil
			}
		}
		position += extensionLen
	}
	return "", nil
}

func parseSNIExtension(extension []byte) (string, bool) {
	if len(extension) < 2 {
		return "", false
	}
	listLen := int(extension[0])<<8 | int(extension[1])
	position := 2
	end := min(position+listLen, len(extension))
	for position+3 <= end {
		nameType := extension[position]
		nameLen := int(extension[position+1])<<8 | int(extension[position+2])
		position += 3
		if position+nameLen > end {
			return "", false
		}
		if nameType == 0 {
			return string(extension[position : position+nameLen]), true
		}
		position += nameLen
	}
	return "", false
}
