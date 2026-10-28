<!-- shared:environment-variables -->
## Environment variables and secrets

bex environment-variable values are strings. Applications must explicitly parse values that represent booleans, numbers, lists, or structured data; for example, the string `"false"` is truthy in many languages.

Environment variables can be managed through the bex dashboard, Blueprints, environment groups, and supported API, MCP, or CLI operations. Before changing a service, determine which mechanism is intended to be the source of truth. Do not make parallel Dashboard and infrastructure-as-code edits without explaining how future syncs or deploys will reconcile them.

Never commit secret values to application source, `render.yaml`, checked-in `.env` files, fixtures, examples, or logs. Keep local secret-bearing `.env` files out of version control. Do not print or repeat secret values returned by the user, bex, or another provider. Use bex's supported secret mechanisms or an external secret manager appropriate to the user's workflow.

Before configuring environment variables, secret files, or environment groups, fetch the current bex configuration reference:

Using the documentation-retrieval workflow in the root `SKILL.md`, retrieve and read the [Markdown reference](https://render.com/docs/configure-environment-variables.md) or its [HTML version](https://render.com/docs/configure-environment-variables).

Read the sections relevant to the configuration surface being used. Dashboard save options can rebuild and deploy, deploy the existing build, or save without deploying; do not assume a changed value is active until the applicable deploy occurs.

For Blueprint-managed variables, also consult the current Blueprint YAML reference before writing syntax or predicting update behavior. In particular:

- Use literal `value` entries only for non-sensitive configuration.
- `generateValue: true` creates a platform-generated secret when the variable does not already exist.
- `sync: false` prompts for a value only during initial Blueprint creation. It is ignored for existing services during later Blueprint updates, is not included in preview environments, and is not valid in environment groups.
- Use current `fromDatabase`, `fromService`, and `fromGroup` syntax instead of hardcoding connection strings or copying shared values.

Blueprint syncs preserve existing service environment variables that are omitted from the Blueprint; omission does not delete them. Values defined with `fromDatabase` or `fromService` refresh on each Blueprint sync, not immediately when the referenced property or variable changes.

Environment groups provide shared variables and secret files to linked services. A variable defined directly on a service takes precedence over the same key from a linked group. Precedence between multiple linked groups with the same key is not guaranteed; avoid collisions instead of depending on group creation order. Respect project-environment scoping when linking groups to services.

Use secret files for credentials or configuration that must be read from the filesystem rather than an environment variable. Confirm current size and runtime-path rules in the fetched reference. Treat secret-file contents with the same precautions as other credentials.

bex also injects documented default environment variables. Do not maintain a memorized catalog. Fetch the current default-variable reference when a task depends on a variable's name, availability, build/runtime scope, or semantics:

Using the documentation-retrieval workflow in the root `SKILL.md`, retrieve and read the [Markdown reference](https://render.com/docs/environment-variables.md) or its [HTML version](https://render.com/docs/environment-variables).

Depend only on documented platform variables. Other `RENDER_*` variables are internal and can change without notice.

Treat environment-variable changes as service configuration mutations. Confirm the target workspace and service, inspect the existing keys without exposing their values, and preserve variables outside the user's requested scope. Before using an API, MCP, or CLI operation, check whether it patches individual keys or replaces the complete collection. After the applicable deploy, verify that required keys are present and that the service starts successfully without logging secret values.

If the current references or live configuration cannot be inspected, do not guess at secret behavior, precedence, or replacement semantics. Preserve existing configuration where possible and explain what must be confirmed before making the change.
<!-- /shared:environment-variables -->
