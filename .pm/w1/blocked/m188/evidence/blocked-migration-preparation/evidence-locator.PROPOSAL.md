# Evidence locator after blocked migration — PROPOSAL ONLY

Root will fill the actual migration date and create this human note after the move. This draft does not assert that migration or the milestone DoD is complete.

- The old `.pm/w1/m188/` prefix now maps to `.pm/w1/blocked/m188/`; completed task paths keep their existing `done/` component. Only t002 may newly move into task `done/`. The milestone and t003–t008 remain open behind qualified common integration.
- Treat an original path together with its expected SHA-256 and byte length as the identity. A later file at the same historical `/tmp` path is not a substitute. Use the retained source-to-archive mapping/content locator to find the matching bytes; never rewrite original evidence strings.
- Resolve each `archive_path` from its sibling MANIFEST directory. This remains valid when the entire milestone later moves between `blocked/m188` and `done/m188`; preserve the internal tree and every evidence byte, including Markdown.
- For gzip records, `stored_*`/`gzip_*` describe the file on disk; `original_*`/`raw_*` describe the decompressed original. Verify the appropriate hash/length at each layer. Do not normalize whitespace, indentation or line endings.
- Large signed predicates remain at the original immutable public OCI objects and verified local gzip custody referenced by their existing receipts. This locator neither reconstructs signed statements nor claims a fresh download/verification, common adoption or hosted acceptance.
