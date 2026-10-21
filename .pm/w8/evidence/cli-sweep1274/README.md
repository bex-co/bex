# CLI sweep 1274 — Render's official Django example (`render-examples/django`) as a Blueprint (live, 2026-10-21)

Production `https://api.bex.co/v1/`, pin `9cace6b6e25d`, workspace `tian-personal`, human device-login grant, HEAD-built pinned CLI. Blueprint `blp-dbc6m7m5u6js73eso4fg` created free Postgres `dpg-dbc6m7m5u6js73eso4gg` and free Python web `srv-dbc6m7u5u6js73eso4i0` (`mysite`). Its `buildCommand` is `./build.sh` (pip install, collectstatic, `manage.py migrate`) and `DATABASE_URL` comes from `fromDatabase … connectionString`.

| Deploy | Change before it | Result |
| --- | --- | --- |
| 1 (first sync) | — | `build_failed` in 214 s: `psycopg2/_psycopg.cpython-313…so: undefined symbol: _PyInterpreterState_Get`. The repo pins `psycopg2-binary==2.9.9`, which cannot load on 3.13+ (Render's current default is 3.14). This is the repo's own problem, not bex's |
| 2 | `PYTHON_VERSION=3.12.11` (REST env PUT) | `build_failed`: `collectstatic` → `You're using the staticfiles app without having set the STATIC_ROOT setting`. The cause is `DEBUG = 'RENDER' not in os.environ`, and bex sets no `RENDER` → **w8/159** |
| 3 | `RENDER=true`, `RENDER_EXTERNAL_HOSTNAME=mysite.onbex.co` | `build_failed`: `manage.py migrate` → `connection to server at "localhost" … port 5432 failed: Connection refused`, because `DATABASE_URL` is absent from the build env → **w8/158** |
| 4 | `buildCommand` without migrate; `preDeployCommand: python manage.py migrate` | `live` (created 07:52:24, finished 07:57:56). `GET /` 200, `/admin/login/` 200 with static CSS, `/nonexistent-qa-path` production 404 |

With those three user-side workarounds the stack works on bex: Postgres via `fromDatabase`, migrations via a free-plan pre-deploy, and static files via whitenoise.

**Cleanup:** Blueprint 204 → 404; service and Postgres 204 → 404.
