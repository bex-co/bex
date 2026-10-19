# CLI sweep 578 — REST secret-files: single/bulk writes reach the mounted file

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `77ec448ae`, production pin `d98d07f62`, 2026-10-19T20:29–20:40Z. Owned fixture `qa578-secret` (`srv-dbb7rhno17is73furtr0`, free `busybox:1.36`) printed `/etc/secrets/qa578.txt` between markers at startup, then served HTTP 200. Same-credential REST matrix in `/tmp/qa501/secrets578.py`; fixture deleted with service, secret-files, and URL all returning 404. Only harmless marker contents were used.

Setup note: the image service initially had no runnable command. A flat `serviceDetails.dockerCommand` PATCH was correctly refused as an unknown field; the pinned nesting `serviceDetails.envSpecificDetails.dockerCommand` returned 200, the superseding deploy went `live`, and the startup log first proved the file was absent.

- **Single writes reach the process:** `PUT qa578.txt` (`contenuto-v1`) and three other accepted files each opened a deploy; after settle the startup log printed the exact content between `SF-BEGIN`/`SF-END`. Updating the same file to a two-line unicode value produced a new deploy and the process printed both lines byte-identically.
- **Read/delete shapes:** single PUT echoes flat `{name, content}` with 201; GET one returns the same pair; list after the singles was `[altro578.txt, empty578.txt, qa578.txt, vuoto578.txt]`. DELETE of an existing file returned 204; a subsequent GET and a GET of a never-created name both returned `404 secret file not found`.
- **Bulk replace is real (the w8/063-era gap stays fixed):** `PUT [{bulk578.txt}]` returned the cursor-wrapped singleton and the immediate list contained only it; the next deploy's process saw only the replacement set. `PUT []` returned `[]` and emptied the list.
- **Name/content dialects:** encoded traversal, encoded slash, and an encoded space all return the named invalid-config-key grammar; 254 characters returns the named 253-character cap; numeric content returns the named body-schema 400. A raw slash in the URL is simply no route (404), while `%2F` reaches validation and gets the named 400. A leading-dot name (`.hidden578`) is accepted by the config-key grammar and lists normally. Omitting `content` creates an empty file, indistinguishable from an explicit empty string.

## Unrun

- Content-size ceiling and binary/non-UTF8 content.
- Env-group secret-file routes and a linked service inheriting a group file.
- The mounted permissions/mode of `/etc/secrets/*` inside the container (content was observed; metadata was not).
