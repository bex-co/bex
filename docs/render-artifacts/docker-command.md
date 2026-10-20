# Render Docker Command: how the string becomes a process

Captured on 2026-10-17 and 2026-10-19 from live Render free web services (deleted afterwards), for w6/077. The probes were:

- `docker.io/library/python:3.12-slim` running a script that printed `/proc/1/cmdline`;
- `docker.io/library/busybox:1.36` running `printf`, which prints each argument on its own line.

## Render's behavior (measured)

| Docker Command (as typed) | What the process received | Evidence |
| --- | --- | --- |
| `printf <%s>\n one "two words" 'single quoted' $HOME a\ b x&&y ;z` | argv `printf`, `<%s>n`, `one`, `two words`, `single quoted`, `$(HOME)`, `a b`, `x&&y`, `;z` | the printf output, one line per argument |
| `sh -c printf "<%s>\n" one "two words" $PORT` | `sh`, `-c`, then the **rest verbatim**: `printf "<%s>\n" one "two words" $PORT`. sh printed `<one>`, `<two words>`, `<10000>` | the printf output |
| `sh -c "echo $P \| base64 -d > /tmp/p.py; exec python3 /tmp/p.py"` | `sh`, `-c`, `"echo <P's value> \| … /tmp/p.py"` **with the quote characters**. sh read it as one quoted word: `File name too long` | the 2026-10-17 privilege probe |
| `python3 -c exec(…) "two words" 'single quoted' …` | `python3`, `-c`, then the rest verbatim, quotes included: Python `SyntaxError` | the argv probe |
| `python3 -c exec(…)` | runs | the 2026-10-17 privilege probe |

Rules inferred from these results:

1. **No shell.** The command becomes the container's argv. `&&`, `;` and `|` are ordinary characters.
2. **`<program> -c <rest>`:** the rest is passed through verbatim as one argument, quote characters included. A quoted `sh -c "…"` therefore fails, and only the unquoted form `sh -c cmd args` works.
3. **Otherwise words are split shell-style.** Double and single quotes group words and are removed, and a backslash escapes the next character (so `\n` becomes `n`).
4. **`$VAR` is rewritten to `$(VAR)`**, Kubernetes' container-args syntax. A variable defined in the service's environment is expanded (`$PORT` → `10000`); any other stays as the literal `$(HOME)`.

## bex's behavior

The operator runs the Docker Command as `sh -c <command>` (`shellCommand`, `lego/operator/internal/controller/deployment_projection.go:217`; pre-deploy commands too, ADR004 `:268`). The whole string is shell syntax.

| Command | Render | bex |
| --- | --- | --- |
| `./start.sh --port 8000`, `node server.js` | runs | runs (same argv) |
| `gunicorn app:app --bind 0.0.0.0:$PORT` | runs (`$(PORT)` expanded by Kubernetes) | runs (expanded by sh) |
| `sh -c "npm run migrate && npm start"` | **fails** (quotes reach sh) | runs |
| `npm run migrate && npm start` | runs `npm` with `&&` as an argument | chains the two commands |
| `python3 -c print(1)` | runs | **fails** (sh syntax error on `(`) |

## Decision (w6/077, 2026-10-19)

The divergence is documented and bex's `sh -c` is kept. For plain commands and `$VAR` expansion the two platforms agree. Where they differ, bex runs the shell idioms Render users actually write: quoted `sh -c "…"` scripts and `&&` chains. Render runs those differently or not at all.

Matching Render would change the meaning of commands existing bex services already run. The opposite case is a command meant literally that contains shell syntax (`(`, `;`, `&`, `|`) without quoting. It works on Render and fails on bex, and it is rare. No single spelling of such a command behaves the same on both platforms: Render passes quote characters after `-c` through. A migrating user quotes it for bex, for example `python3 -c 'print(1)'`.

Revisit this with a per-service launch-mode field if a migrating user hits the difference. Existing services must keep `sh -c`.
