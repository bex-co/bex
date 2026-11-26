# Setting your Bun Version

> For the complete documentation index, see [llms.txt](https://render.com/llms.txt).


| Current default Bun version |
| --- |
| *`1.4.5`* Services created before *2026-11-20* have a different default version. [See below](<https://render.com/docs/bun-version.md#history-of-default-bun-versions>). |

> *To include Bun in your service's environment, you must do _at least one_ of the following:*
>
> - Set your service's Bun version using one of the methods below.
> - Include a `bun.lock` or `bun.lockb` file in your service's root directory.
>
> Otherwise, your service's environment will _not_ include Bun.

*Set your service's Bun version in any of the following ways* (in descending order of precedence):

1. Set the `BUN_VERSION` environment variable for your service in the [Render Dashboard](https://dashboard.render.com):

   [image: Setting the BUN_VERSION environment variable]

2. Add a file named `.bun-version` to the root of your repo. This file contains a single line with the version to use:

   ```
   1.4.5
   ```

## Supported version values

| Value | Description |
| --- | --- |
| A semantic version number (such as `1.4.5`) | Uses the exact Bun version specified for every deploy. Recommended for most services. |
| `latest` | Uses the most recent [stable release](https://github.com/oven-sh/bun/releases) of Bun for every deploy. |
| `canary` | Uses the most recent [canary build](https://bun.com/docs/installation#canary-builds) of Bun for every deploy. |

> *Using `latest` or `canary` might introduce breaking changes in your service.*
>
> - Because these versions change over time, a future release might be incompatible with your service's code or configuration.
> - Never use `canary` for a production service. Canary builds are untested pre-releases.
>     - Instead, use `canary` in non-production services to preview upcoming Bun changes.

## History of default Bun versions

If you don't set a Bun version for your service, Render's default version depends on when you originally created the service:

| Service Creation Date | Default Bun Version |
|---|---|
| 2026-11-20 and later | `1.4.5` |
| 2026-10-22 | `1.4.4` |
| 2026-10-12 | `1.4.3` |
| 2026-09-09 | `1.4.2` |
| 2026-07-16 | `1.3.14` |
| 2025-12-08 | `1.3.4` |
| 2025-08-18 | `1.2.20` |
| Before 2025-08-18 | `1.1.0` |
