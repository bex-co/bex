#!/usr/bin/env python3
# Copyright 2026.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Minimal dependency-free ACP-over-WebSocket initialize probe for m37."""

import base64
import json
import os
import socket
import struct
import sys


def receive_exact(connection: socket.socket, length: int) -> bytes:
    chunks = bytearray()
    while len(chunks) < length:
        chunk = connection.recv(length - len(chunks))
        if not chunk:
            raise RuntimeError("WebSocket closed before a complete frame arrived")
        chunks.extend(chunk)
    return bytes(chunks)


def receive_frame(connection: socket.socket) -> tuple[int, bytes]:
    first, second = receive_exact(connection, 2)
    opcode = first & 0x0F
    length = second & 0x7F
    if length == 126:
        length = struct.unpack("!H", receive_exact(connection, 2))[0]
    elif length == 127:
        length = struct.unpack("!Q", receive_exact(connection, 8))[0]
    if second & 0x80:
        mask = receive_exact(connection, 4)
        payload = receive_exact(connection, length)
        return opcode, bytes(value ^ mask[index % 4] for index, value in enumerate(payload))
    return opcode, receive_exact(connection, length)


def send_text(connection: socket.socket, payload: bytes) -> None:
    mask = os.urandom(4)
    length = len(payload)
    if length < 126:
        header = bytes((0x81, 0x80 | length))
    elif length <= 0xFFFF:
        header = bytes((0x81, 0x80 | 126)) + struct.pack("!H", length)
    else:
        header = bytes((0x81, 0x80 | 127)) + struct.pack("!Q", length)
    masked = bytes(value ^ mask[index % 4] for index, value in enumerate(payload))
    connection.sendall(header + mask + masked)


def main() -> None:
    if len(sys.argv) != 3:
        raise SystemExit("usage: m37-acp-ws-probe.py <host> <port>")
    host, port = sys.argv[1], int(sys.argv[2])
    with socket.create_connection((host, port), timeout=10) as connection:
        key = base64.b64encode(os.urandom(16)).decode()
        request = (
            f"GET /acp HTTP/1.1\r\nHost: {host}:{port}\r\n"
            "Connection: Upgrade\r\nUpgrade: websocket\r\n"
            f"Sec-WebSocket-Key: {key}\r\nSec-WebSocket-Version: 13\r\n\r\n"
        )
        connection.sendall(request.encode())
        response = bytearray()
        while b"\r\n\r\n" not in response:
            response.extend(connection.recv(4096))
        if not response.startswith(b"HTTP/1.1 101 "):
            raise RuntimeError("server rejected the WebSocket upgrade")

        send_text(
            connection,
            json.dumps(
                {
                    "jsonrpc": "2.0",
                    "id": 1,
                    "method": "initialize",
                    "params": {
                        "protocolVersion": 1,
                        "clientCapabilities": {
                            "fs": {"readTextFile": False, "writeTextFile": False},
                            "terminal": False,
                        },
                        "clientInfo": {"name": "bex-m37-probe", "version": "1"},
                    },
                },
                separators=(",", ":"),
            ).encode(),
        )
        while True:
            opcode, payload = receive_frame(connection)
            if opcode == 0x8:
                raise RuntimeError("agent closed before initialize completed")
            if opcode != 0x1:
                continue
            message = json.loads(payload)
            if message.get("id") != 1:
                continue
            if message.get("error") or not message.get("result", {}).get("agentInfo"):
                raise RuntimeError("ACP initialize returned no agentInfo")
            print(message["result"]["agentInfo"]["name"])
            return


if __name__ == "__main__":
    main()
