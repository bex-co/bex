#!/usr/bin/env python3
"""Exercise the installed system decoder; no package-version-only verdict."""
import ctypes as c
import subprocess
from pathlib import Path


def check_system_decoder():
    decoder = c.CDLL("libbrotlidec.so.1")
    decoder.BrotliDecoderCreateInstance.argtypes = [c.c_void_p] * 3
    decoder.BrotliDecoderCreateInstance.restype = c.c_void_p
    decoder.BrotliDecoderDestroyInstance.argtypes = [c.c_void_p]
    decoder.BrotliDecoderGetErrorCode.argtypes = [c.c_void_p]
    decoder.BrotliDecoderDecompressStream.argtypes = [
        c.c_void_p, c.POINTER(c.c_size_t), c.POINTER(c.c_void_p),
        c.POINTER(c.c_size_t), c.POINTER(c.c_void_p), c.POINTER(c.c_size_t),
    ]
    # Official Node regression, b5f41529df78a8af7b395abd6a97d2dcb6c52c61.
    # Only the malformed header: no overflow payload or exploit machinery.
    malformed = bytes.fromhex("200010414141fcffff07002228f00b60f7f41f000000")
    for chunks in ([malformed], [bytes([b]) for b in malformed]):
        state = decoder.BrotliDecoderCreateInstance(None, None, None)
        if not state:
            raise RuntimeError("could not allocate Brotli decoder")
        try:
            code = 0
            produced = 0
            for chunk in chunks:
                source = c.create_string_buffer(chunk)
                available_in = c.c_size_t(len(chunk))
                next_in = c.c_void_p(c.addressof(source))
                while True:
                    target = c.create_string_buffer(65536)
                    available_out = c.c_size_t(len(target))
                    next_out = c.c_void_p(c.addressof(target))
                    result = decoder.BrotliDecoderDecompressStream(
                        state, c.byref(available_in), c.byref(next_in),
                        c.byref(available_out), c.byref(next_out), None,
                    )
                    produced += len(target) - available_out.value
                    if produced > 64 * 1024 * 1024:
                        raise RuntimeError("malformed stream exceeded regression output bound")
                    # 1.0.x emits 16 MiB before reaching the invalid block
                    # switch. Drain output without dropping unconsumed input.
                    if result != 3:  # BROTLI_DECODER_RESULT_NEEDS_MORE_OUTPUT
                        break
                code = decoder.BrotliDecoderGetErrorCode(state)
                if code < 0:
                    break
            if code != -17:
                raise RuntimeError("system Brotli did not reject invalid block switch: %s" % code)
        finally:
            decoder.BrotliDecoderDestroyInstance(state)
    # Keep a healthy round trip beside the malformed-stream check.
    encoder = c.CDLL("libbrotlienc.so.1")
    encoder.BrotliEncoderCompress.argtypes = [
        c.c_int, c.c_int, c.c_int, c.c_size_t, c.c_void_p,
        c.POINTER(c.c_size_t), c.c_void_p,
    ]
    original = b"bex native runtime Brotli compatibility\n" * 4
    encoded = c.create_string_buffer(1024)
    encoded_size = c.c_size_t(len(encoded))
    if not encoder.BrotliEncoderCompress(6, 22, 0, len(original), original, c.byref(encoded_size), encoded):
        raise RuntimeError("Brotli compression failed")
    decoder.BrotliDecoderDecompress.argtypes = [c.c_size_t, c.c_void_p, c.POINTER(c.c_size_t), c.c_void_p]
    output = c.create_string_buffer(1024)
    output_size = c.c_size_t(len(output))
    if decoder.BrotliDecoderDecompress(encoded_size.value, encoded, c.byref(output_size), output) != 1:
        raise RuntimeError("valid Brotli stream did not decode")
    if output.raw[:output_size.value] != original:
        raise RuntimeError("valid Brotli stream changed contents")


if __name__ == "__main__":
    check_system_decoder()
    for binary in ("/usr/local/bin/node", "/opt/bex/native-node-tools/node"):
        if Path(binary).is_file():
            subprocess.run([binary, "/usr/share/bex-security/node-check.js"], check=True, timeout=30)
    print("system and bundled Brotli checks passed")
