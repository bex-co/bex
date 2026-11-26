# Setting Your uv Version

> For the complete documentation index, see [llms.txt](https://render.com/llms.txt).


| Current default uv version |
| --- |
| *`0.13.7`* |

[uv](https://docs.astral.sh/uv/) is a packaging and dependency manager for Python. Render automatically adds uv to your Python service's runtime if your project's root directory includes a `uv.lock` file.

To specify a uv version, set your service's `UV_VERSION` [environment variable](<https://render.com/docs/configure-environment-variables.md>) to any version number that's compatible with your [Python version](<https://render.com/docs/python-version.md>).

## History of default uv versions

| Service Creation Date | Default uv Version |
|---|---|
| 2026-11-03 and later | `0.13.7` |
| 2026-10-12 | `0.12.24` |
| 2026-09-09 | `0.12.10` |
| 2026-02-11 | `0.10.2` |
| Before 2025-06-12 | `0.7.12` |
