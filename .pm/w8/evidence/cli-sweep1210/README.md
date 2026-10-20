# CLI sweep 1210 — pin advanced to `94533d795067`; census clean (live, 2026-10-20)

Read-only census plus refusal-side retests. No new bug filed.

- Production pin advanced: `f641abe92` pins image `94533d795067`, whose only code change over the prior pin is the private-service TCP-port publication feature (`94533d795`). The port-below-1024 fix (`b01debc5d`) is verified **not** an ancestor of this image.
- At the new pin: private-service create on free still refuses 400 (paid gate intact; the new networking feature itself needs a paid private service, so it stays untested live), and port `80` still refuses 400 as expected without the fix.
- Census: no `qa1*` leftovers in any family; baseline service reads 200.
