#!/bin/sh
# Assembles one official Python line for the agent image (w4/m544), run in
# that line's own digest-pinned python:<line>-bookworm stage of the Dockerfile:
# the interpreter, its pip script, libpython, the standard library (with pip)
# and the headers, under /out/usr/local for one linked COPY. The official
# image deletes every .pyc, and a session's uid cannot write bytecode next to
# the root-owned library, so it is compiled here, unchecked against source
# timestamps since the library never changes. A module that does not compile
# is only slower to import, so that is a warning, not a failed build. The
# full image's libpython and extension modules keep their debug symbols,
# which a session never needs, so they are stripped, with the same leniency.
set -eu
line="$1"
src=/usr/local
out=/out/usr/local
"$src/bin/python$line" -m compileall -q -j0 --invalidation-mode unchecked-hash "$src/lib/python$line" >/dev/null ||
	echo "warning: some of Python $line's modules were not byte-compiled" >&2
mkdir -p "$out/bin" "$out/lib" "$out/include"
cp -a "$src/bin/python$line" "$src/bin/pip$line" "$out/bin/"
cp -a "$src/lib/libpython$line.so.1.0" "$out/lib/"
ln -s "libpython$line.so.1.0" "$out/lib/libpython$line.so"
cp -a "$src/lib/python$line" "$out/lib/"
cp -a "$src/include/python$line" "$out/include/"
find "$out" -type f \( -name "python$line" -o -name '*.so' -o -name '*.so.*' \) -exec strip --strip-debug {} + ||
	echo "warning: some of Python $line's binaries kept their debug symbols" >&2
