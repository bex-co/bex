# Runtime-version fixtures

Native-runtime builds pick their toolchain from Render's documented signals: an env var, then a version file, then (Node only) `package.json` `engines.node`. These two services carry **only** a version file, set to a non-default line, so a build log proves the file was honored. See w8/m51 and `lego/operator/internal/build/nativeversion.go`.

| Directory | File | Expected build narration |
| --- | --- | --- |
| `python/` | `.python-version` = `3.12` (default is 3.13) | `==> Using Python 3.12 (from .python-version)` |
| `node/` | `.nvmrc` = `22` (default is 24) | `==> Using Node 22 (from .nvmrc)` |

Create either as a native service with the repo pointed at this checkout and the root directory set to the fixture. For example, with the bex CLI:

```sh
bex services create --name rv-python --type web_service --runtime python \
  --repo https://github.com/bex-co/bex --branch main \
  --root-directory examples/runtime-version/python \
  --build-command "python --version && pip install -r requirements.txt" \
  --start-command "python main.py"
```

For Node, use `--runtime node`, `--root-directory examples/runtime-version/node`, `--build-command "node --version"` and `--start-command "node main.js"`. Don't set `PYTHON_VERSION` or `NODE_VERSION`, because they would take precedence over the file.
