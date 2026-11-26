# Setting Your Python Version

> For the complete documentation index, see [llms.txt](https://render.com/llms.txt).


> *Issues deploying your Python app?* See [Troubleshooting Python Deploys](<https://render.com/docs/troubleshooting-python-deploys.md>).

| Current default Python version | Minimum supported Python version |
| --- | --- |
| *`3.14.10`* Services created before *2026-11-03* have a different default version. [See below](<https://render.com/docs/python-version.md#history-of-default-python-versions>). | *`3.7.3`* |

*Set a different Python version in _any_ of the following ways* (in descending order of precedence):

1. Set your service's `PYTHON_VERSION` [environment variable](<https://render.com/docs/configure-environment-variables.md>) to a _fully qualified_ Python version (e.g., `3.13.5`). You can specify any released version from `3.7.3` onward.

   [image: Setting the PYTHON_VERSION environment variable]

   You _must_ specify a fully qualified version (e.g., `3.13.5`) if you use this method.

2. Add a file named `.python-version` to the root of your repo. This file contains a single line with the version to use:

   ```
   3.13.5
   ```

   You _can_ omit the patch version (e.g., `3.13`) if you use this method. If you omit it, Render uses the latest corresponding patch version.

Render doesn't support unreleased Python versions natively, but you can use them via [Render's Docker support](<https://render.com/docs/docker.md>).

## Versioning package managers

You can also set versions for the following Python package managers:

| Package manager | Environment variable |
| --- | --- |
| [uv](<https://render.com/docs/uv-version.md>) | `UV_VERSION` 
> To use uv, a `uv.lock` file must be present in your service's root directory. [Learn more](<https://render.com/docs/uv-version.md>).
 |
| [Poetry](<https://render.com/docs/poetry-version.md>) | `POETRY_VERSION` |

## History of default Python versions

If you don't set a Python version for your service, Render's default version depends on when you originally created the service:

| Service Creation Date | Default Python Version |
|---|---|
| 2026-11-03 and later | `3.14.10` |
| 2026-10-12 | `3.14.8` |
| 2026-02-11 | `3.14.3` |
| 2025-06-12 | `3.13.4` |
| 2024-12-16 | `3.11.11` |
| 2024-10-29 | `3.11.10` |
| 2024-04-04 | `3.11.9` |
| 2024-02-22 | `3.11.8` |
| 2024-01-02 | `3.11.7` |
| 2023-12-04 | `3.11.6` |
| Before 2023-11-01 | `3.7.10` |
