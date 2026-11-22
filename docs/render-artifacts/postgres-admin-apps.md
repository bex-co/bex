# Postgres ▸ Apps (pgAdmin, PgHero) — Render's dashboard bundle, 2026-11-21

## Sources

Read without signing in, on 2026-11-21, from Render's public dashboard bundle (`https://dashboard.render.com/` → `assets/index-BR2q8yex.js`) and the chunks it imports, for w4/m551. The hashes change with every Render deploy, so find the code by the strings quoted below:

- `index-BR2q8yex.js`: the database routes (`{path: "apps", …}` beside `info`, `recovery`, `metrics`, `logs`, `plan`), the navigation entry (`"/apps": "Apps"`), the app catalog (`id: "pgadmin"`, `id: "pghero"`), the two `installInput` builders, the deploy dialog (`data-testid: "pg-apps-modal"`) and the deployed card (`"Open app"`).
- `utils-rgkfHmbc.js`: the credential generator (`{appName, username, password}`).
- `crypto-CZWu1usC.js`: the random-string helper the generator calls (32 hex characters by default).
- `owner-Cx7VMOVe.js`: the HIPAA check that picks PgHero's billing line.
- `server-8p_bC2pd.js`: `mutation createServer($server: ServerInput!)`. `service-TN0K5kvW.js`: `query servicesLiteForOwner($ownerId: String!) { servicesForOwner(ownerId: $ownerId) { id slug } }`.
- [render.com/docs/postgresql-apps](https://render.com/docs/postgresql-apps) and the changelog entry of 2024-06-04, "Quickly deploy PostgreSQL admin apps from the Render Dashboard".
- [github.com/render-examples/pgadmin](https://github.com/render-examples/pgadmin) (`Dockerfile`, `render-entrypoint.sh`), the repository Render builds pgAdmin from. `render-examples/pghero` does not exist (GitHub 404): PgHero is an image.

## Render's behaviour

### Where the page is

- The database page's navigation lists **Apps** under **Manage**, after **Plan** and before **Sharing** and **Recovery**: `r && {title: "Apps", href: …/apps, IconStart: Apps}`, where `r` is `role === PRIMARY`. A read replica has no Apps page. No database plan or status hides it.
- The page is titled **Apps** and shows one card per catalog app side by side (stacked on a narrow screen). While the workspace's service list loads it shows a spinner.

### The catalog

|  | pgAdmin | PgHero |
| --- | --- | --- |
| `id` | `pgadmin` | `pghero` |
| Card description | _General-purpose PostgreSQL administration. Manage schemas, tables, and indexes. Run one-off queries and view their query plans._ | _A performance dashboard for your database. Monitor resource usage, analyze active connections, and inspect recent queries to identify bottlenecks._ |
| **Read the docs** link | `https://www.pgadmin.org/docs/` | none |
| Source | GitHub repository `render-examples/pgadmin`, branch `main`, runtime `docker`, `dockerFilePath: ./Dockerfile` | image `docker.io/ankane/pghero:latest` (Docker Hub), runtime `image` |
| Plan | `starter` | `free` |
| Disk | `pgadmin`, 1 GB at `/var/lib/pgadmin` | none |
| Port | `PORT` and `PGADMIN_LISTEN_PORT` = `10000` | none set (Render detects it; the image listens on 8080) |
| Billing lines | _Web service - starter instance_ $7 / month; _Disk - 1GB_ $0.25 / month | _Web service - Free instance_ $0 / month; in a HIPAA workspace _Web service - starter instance_ $7 / month (the builder still sends `plan: free`) |
| Login | `PGADMIN_DEFAULT_EMAIL` / `PGADMIN_DEFAULT_PASSWORD` | `PGHERO_USERNAME` / `PGHERO_PASSWORD` |

### The service each app creates

Both are `createServer` with `autoDeploy: true`, `buildCommand: ""`, the database's `ownerId`, `environmentId` (the database's environment, or none) and `region`, and a name of `${app.name}-${database.id}`: `pgAdmin-dpg-…` or `PgHero-dpg-…`.

**PgHero** env vars, all literals:

- `DATABASE_URL`: the database's **internal** connection string, assembled in the browser from the password the dialog loaded (`connectionString({database, password, internal: true})`).
- `PGHERO_USERNAME`, `PGHERO_PASSWORD`: the dialog's two fields.
- `WEB_CONCURRENCY` = `2`.

**pgAdmin** env vars and secret files:

- `GUNICORN_THREADS` = `2`; `PGADMIN_DATABASE_HOST` = the database id (Render's internal hostname); `PGADMIN_DATABASE_NAME` = the database's name; `PGADMIN_DATABASE_USER` = its user; `PGADMIN_DEFAULT_EMAIL`, `PGADMIN_DEFAULT_PASSWORD` = the dialog's two fields; `PGADMIN_LISTEN_PORT` = `10000`; `PORT` = `10000`; `PGADMIN_SERVER_JSON_FILE` = `/var/lib/pgadmin/storage/servers.json`.
- Secret file `pgpassfile`: `<database id>:5432:*:<user>:<password>`.
- Secret file `servers.json`: `{"Servers":{"1":{"Name":<database name>,"Group":"Servers","Port":5432,"Username":<user>,"PassFile":"pgpassfile","Host":<database id>,"SSLMode":"allow","MaintenanceDB":"postgres"}}}`.
- The repository's image is `FROM dpage/pgadmin4:9.15` run as root: it strips the file capability from `python3` (`setcap -r`), sets `PGADMIN_DISABLE_POSTFIX=true`, and its entrypoint copies `/etc/secrets/servers.json` to `/var/lib/pgadmin/storage/` and `/etc/secrets/pgpassfile` (mode `0600`) into the default user's storage directory, both owned by `pgadmin`, then starts pgAdmin as that user. So pgAdmin opens with the database's server already in its tree and its password saved.

### The credentials

- `generate(appId, "temp")` returns `username = "<appId>-" + 8 random hex characters` (`pghero-1f3a9c0b`) and `password = 32 random hex characters` (`crypto.getRandomValues`).
- PgHero's username is that generated one. **pgAdmin's is the signed-in user's email** (`username: (email, generated) => email`), because pgAdmin logs in by email.
- The dialog's fields are editable password inputs (masked, with a show toggle), prefilled with those values. Render does not validate them in the browser.

### The deploy dialog

- Opening it: **Deploy app** first loads the database's password (the connection-info reveal), with the button spinning; the dialog opens once it is ready. A failed load shows _An error occurred while setting up the database connection. Please try again._ under the button.
- Title **Create a `<app>` app**. Body: _This will deploy a new web service to the same region and environment as your database. You can configure it like any other web service._ **Learn more.** (`/docs/postgresql-apps`).
- **Authentication** — _We’ve automatically generated a username and password for your **`<app>`** instance. You can customize these below._ — then **Username** and **Password** rows.
- **Monthly charges** — _Web service instances are billed and prorated by the second. The charges below only reflect the charges for **`<app>`**._ — then one line per billing item, `<name>` … `$<price> / month`.
- Buttons **Cancel** and **Deploy `<app>`** (both disabled while it sends).
- Success: a toast _**`<app>`** deployed to [`<service name>`](/web/<service id>)._, the workspace's service list is refetched, and the dialog closes.
- A payment-required refusal opens Render's add-card modal with _Add a payment method to this workspace to deploy `<app>`._; once a card is added the same submission is sent again. Any other failure shows _Error: Unable to create the app. Try again?_ in the dialog.

### Deployed state

- An app is **deployed** for a database when some service of the workspace has a slug containing `${app.id}-${database.id}` (`pghero-dpg-…`). The match is by slug, so a renamed app service still counts, and deleting the service brings back **Deploy app**.
- The deployed card shows the app's name with a status badge: **Deployed** (success) when the service is available, running or its deploy succeeded; **Deploying** (animated) while a deploy, an instance update or maintenance runs; otherwise the status label itself (**Suspended**, **Unavailable**, …, warning tone for those two).
- **Open app** (primary, opens the service URL in a new tab) is disabled while deploying, suspended or unavailable. **Manage** (a settings link) opens the service.
- There is no second-app guard on the server: `createServer` takes a second app like any other service. The dashboard never offers one, because the card of a deployed app has no **Deploy app**.

### Who can deploy

- **Deploy app** is enabled when `database.permissions.viewDatabaseConnectionInfo` is `ALLOWED`; otherwise it is disabled inside a tooltip carrying that permission's message. The create itself is `createServer`, under the caller's service-create permission.

### API surface

- The page runs on the dashboard's GraphQL only: `servicesForOwner`, the connection-info read and `createServer`. Render's public REST API and MCP server have no admin-app operation.

## What bex had before w4/m551

- The database page (`dashboard/src/routes/databases.$databaseId.tsx`) had no Apps section; the code and the board mentioned pgAdmin only as a Blueprint fixture (`lego/backend/internal/apps/blueprint_names_test.go`).
- The building blocks existed: image and public-repository services, secret files, disks, a database's internal connection string (`databaseConnectionInfo`) and the shared create path (`apps.Service.Create`).

## What bex has since w4/m551

The page is the database page's **Apps** section, and the service is created server side by one GraphQL verb, `deployPostgresAdminApp`, so the connection string never travels through the browser. The design is [ADR009](../ADR009-postgresql-management.md) § Admin apps; what is still built differently is recorded in [ADR018](../ADR018-render-parity.md)'s Managed Postgres section, row _Postgres admin apps (pgAdmin, PgHero)_.
