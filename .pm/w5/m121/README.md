# w5 · m121 — Custom domains: public-suffix-aware validation, correct DNS record names, one TLS secret name

**Worker:** worker5 **Goal:** bex refuses bare public suffixes, tells users the right CNAME name under multi-label suffixes, and reports long hosts as verified once their certificate exists. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Refuse hosts with no registrable domain | 30m | — |
| t002 | Name CNAME records relative to the registrable domain | 30m | — |
| t003 | One TLS secret-name function in types | 45m | — |
| t004 | Decide how much of the dashboard's parent domain to reserve | 30m | — |
| t005 | Render parity | 20m | t001, t002, t003, t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t005, t006 |
| t008 | Closeout | 10m | t007 |

## Definition of done

- `co.uk` and `github.io` are refused with a coded 400, and `example.co.uk` is accepted.
- `www.example.co.uk` gets CNAME name `www` (zone `example.co.uk`), consistent with the TXT ownership record; apex records are unchanged.
- Backend and operator compute TLS Secret names with the same `types` function, and a long host shows verified once its Secret exists.
- The parent-domain reservation decision is recorded (a user decision if it changes self-host behavior).

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05, of `6f4ca308a` (w4/190). Code-verified at `4baa77e02`: the hostname check only requires a dot; `dnsRecordFor` strips two labels; and `tlsSecretForHost` truncates while the operator hashes.
- **Goal linkage:** ADR005 custom domains; ADR018 parity.
- **Expected outcome:** Correct DNS guidance for every public-suffix domain, and no long hosts stuck as pending.
- **Why now:** w4/190 touched this validation on 2026-10-04, and the wrong record name misleads users on common ccTLDs.
- **Render parity included:** domain errors and DNS instructions change on every surface.
