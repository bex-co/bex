# default-env-probe

The fixture behind w32/m425's capture, `docs/render-artifacts/default-env-live.md`: it prints, at build and at runtime, every environment variable Render documents as a platform default ([render.com/docs/environment-variables](https://render.com/docs/environment-variables)), plus `PORT`. `probe.sh` prints those names only, one line each (`DEFAULT_ENV <phase> NAME=value` or `DEFAULT_ENV <phase> NAME unset`), never the rest of the environment.

Every service below uses the repository `https://github.com/bex-co/bex`, branch `main` and root directory `examples/default-env-probe`, except the Elixir one, and none sets its own variables. Read the lines back with `GET /v1/logs?ownerId=<workspace>&resource=<service id>&type=build` (or `type=app`) and `text=DEFAULT_ENV`.

| Fixture | Type | Runtime | Build command | Start command |
| --- | --- | --- | --- | --- |
| Node | web | `node` | `sh probe.sh build` | `sh probe.sh runtime && exec perl serve.pl 3000` |
| Python | web | `python` | `sh probe.sh build` | `sh probe.sh runtime && exec perl serve.pl` |
| Ruby | web | `ruby` | `sh probe.sh build` | `sh probe.sh runtime && exec perl serve.pl` |
| Go | web | `go` | `sh probe.sh build` | `sh probe.sh runtime && exec perl serve.pl` |
| Rust | web | `rust` | `sh probe.sh build` | `sh probe.sh runtime && exec perl serve.pl` |
| Docker | web | `docker` | the `Dockerfile` (declares every name as an `ARG`) | its `CMD` |
| Private | private | `node` | `sh probe.sh build` | `sh probe.sh runtime && exec perl serve.pl` |
| Worker | worker | `node` | `sh probe.sh build` | `sh probe.sh runtime && exec sleep infinity` |
| Cron | cron | `node` | `sh probe.sh build` | `sh probe.sh runtime` |
| Static | static | — | `sh probe.sh build && mkdir -p public && echo ok > public/index.html` (publish `public`) | — |

- The Node web service listens on 3000 while `PORT` stays 10000, so it also checks that bex detects the port, as Render does (w8/202).
- `serve.pl` is a dependency-free responder: every Debian-based toolchain image ships Perl, so no fixture needs a compile step.
- **Elixir** runs Render's own [`render-examples/phoenix_hello`](https://github.com/render-examples/phoenix_hello) (branch `master`, with a throwaway `SECRET_KEY_BASE`). Its build command downloads `probe.sh` from this directory before `./build.sh`, and its start command runs it before `_build/prod/rel/phoenix_hello/bin/server`.
- **Prebuilt image:** `docker.io/library/busybox:1.37.0` with the Docker Command `sh -c env | grep -E '^(NAME|…)=' | sed 's/^/DEFAULT_ENV runtime /'; exec httpd -f -p 10000 -h /tmp`, listing the same names as `probe.sh`. An image service has no build, and an unset name prints no line.
