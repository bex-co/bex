# Setting Your Poetry Version

> For the complete documentation index, see [llms.txt](https://render.com/llms.txt).


| Current default Poetry version |
| --- |
| *`2.1.3`* Services created before *2025-06-12* have a different default version. [See below](<https://render.com/docs/poetry-version.md#history-of-default-poetry-versions>). |

[Poetry](https://pypi.org/project/poetry/) is a packaging and dependency manager for Python. It's automatically included in Render's native Python runtime.

To specify a Poetry version, set your service's `POETRY_VERSION` [environment variable](<https://render.com/docs/configure-environment-variables.md>) to any version number that's compatible with your [Python version](<https://render.com/docs/python-version.md>).

## History of default Poetry versions

If you don't set a Poetry version for your service, Render's default version depends on when you originally created the service:

| Service Creation Date | Default Poetry Version |
|---|---|
| 2025-06-12 and later | `2.1.3` |
| 2023-11-30 | `1.7.1` |
| 2021-04-27 | `1.1.x (various)` |
| Before 2021-04-27 | `1.0.x (various)` |
