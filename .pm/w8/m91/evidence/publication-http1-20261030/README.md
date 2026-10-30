# Supplier scan HTTP/2 transport correction — 2026-10-30

Node20 and a later duplicate Node14 scan failed while reading different image layers with peer HTTP/2 PROTOCOL_ERROR, below their twenty-minute scanner deadline. Their runtime checks passed; neither failed scan approved a supplier. The first failed Node20 job was rerun in the background. A second matching failure establishes the need for a repository correction.

Both scan steps now set GODEBUG=http2client=0. The pinned Trivy/action/container-registry source uses the inherited standard Go transport; the exact primary-source chain is recorded in the receipt. The local actual TLS probe with the same double-clone pattern negotiates HTTP/2 by default and HTTP/1.1 under the setting. Both retain TLS1.3 and certificate validation, reject an untrusted CA and reject a tampered digest. All scan inputs/platforms, twenty-minute timeouts, reports, signing and existing finding policy remain unchanged.

This proves transport behavior, not a new scanner run or supplier approval. Real corrected workflow runs remain required. Raw failure logs are lossless gzip with original hashes; no credentials are included.
