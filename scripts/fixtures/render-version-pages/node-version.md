# Setting Your Node.js Version

> For the complete documentation index, see [llms.txt](https://render.com/llms.txt).


| Current default Node.js version |
| --- |
| *`24.22.1`* Services created before *2026-10-21* have a different default version. [See below](<https://render.com/docs/node-version.md#history-of-default-nodejs-versions>). |

*Set a different Node.js version in _any_ of the following ways* (in descending order of precedence):

1. Set the `NODE_VERSION` environment variable for your service in the [Render Dashboard](https://dashboard.render.com):

   [image: Setting the NODE_VERSION environment variable]

2. Add a file named `.node-version` to the root of your repo. This file contains a single line with the version to use:

   ```
   20.18.0
   ```

3. Add a file named `.nvmrc` to the root of your repo. This file uses the same format as `.node-version`.

4. Specify a Node.js version range in your `package.json` file, under the [`engines`](https://docs.npmjs.com/cli/v10/configuring-npm/package-json#engines) property:

   ```json
   "engines": {
     "node": ">=20.0.0 <22.0.0"
   }
   ```

   If there isn't a `package.json` file in your repo's root directory, Render uses the first `package.json` file it finds in a subdirectory.

> *Always include an upper bound in your version range.*
>
>    An unbounded range (such as `>=20`) always resolves to the [`latest` release](https://nodejs.org/download/release/latest/) of Node.js, which increments its major version over time. This might result in unexpected behavior or incompatibilities with your development version.

You can specify either a semantic version number (such as `18.18.0`) or an alias (such as `lts`).

> Render uses the [`node-version-alias`](https://github.com/ehmicky/node-version-alias) module to resolve version aliases and [semver](https://semver.org) ranges.

## History of default Node.js versions

If you don't set a Node.js version for your service, Render's default version depends on when you originally created the service:

| Service Creation Date | Default Node.js Version |
|---|---|
| 2026-10-21 and later | `24.22.1` |
| 2026-09-17 | `24.21.0` |
| 2026-04-21 | `24.14.1` |
| 2026-01-14 | `22.22.0` |
| 2025-06-12 | `22.16.0` |
| 2024-12-16 | `22.12.0` |
| 2024-11-24 | `22.11.0` |
| 2024-10-30 | `22.10.0` |
| 2024-07-09 | `20.15.1` |
| 2024-04-17 | `20.12.2` |
| 2024-04-04 | `20.12.1` |
| 2024-03-27 | `20.12.0` |
| 2024-02-23 | `20.11.1` |
| 2023-11-29 | `20.10.0` |
| Before 2023-11-01 | `14.17.0` |
