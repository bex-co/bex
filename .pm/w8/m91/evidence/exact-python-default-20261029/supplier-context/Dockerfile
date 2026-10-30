# syntax=docker/dockerfile:1.7
FROM localhost:32828/native-bookworm@sha256:29e85dec2901b622cb18ef10e2eb594a3de7f642c83d27f533c1c813dc7de2be
RUN apt-get update && apt-get install -y --no-install-recommends libtcl8.6 libtk8.6 && rm -rf /var/lib/apt/lists/*
COPY python-tools.json fetch-python-tools.py install-python-tools.py /opt/bex/python-tools/
COPY bex-native-python-tools /usr/bin/
COPY check-brotli.py /usr/share/bex-security/check-brotli.py
COPY node-brotli/check.js /usr/share/bex-security/node-check.js
RUN chmod 755 /usr/bin/bex-native-python-tools && /usr/bin/python3 /opt/bex/python-tools/fetch-python-tools.py
RUN --network=none /usr/bin/bex-native-python-tools && /usr/bin/python3 /usr/share/bex-security/check-brotli.py
