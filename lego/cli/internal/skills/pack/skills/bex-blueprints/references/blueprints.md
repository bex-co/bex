<!-- shared:blueprints -->
## bex Blueprints

A bex Blueprint is a declarative YAML definition of interconnected bex services, databases, and environment groups. The file is usually named `render.yaml` and stored at the root of a Git repository, although its path can be customized when the Blueprint is created.

Blueprint creation and sync read the file from the linked remote Git repository. Local edits are not available to bex until they are committed and pushed to that repository.

Do not rely on remembered Blueprint fields, allowed values, defaults, or update behavior. Before creating or modifying a Blueprint, use the documentation-retrieval workflow in the root `SKILL.md` to retrieve and read the [Markdown reference](https://render.com/docs/blueprint-spec.md) or its [HTML version](https://render.com/docs/blueprint-spec).

If neither version can be retrieved, use the current JSON Schema and bex validation where possible. Use the bundled guidance only for stable workflow and safety constraints, and do not guess at unfamiliar or potentially changed syntax.

Read the sections relevant to every resource and field being changed. In particular, confirm required fields, service-type support, immutable fields, defaults for new resources, behavior for existing resources, and whether omitting or emptying a field preserves or removes existing configuration. Do not treat an example Blueprint as a complete specification.

Use the public JSON Schema for editor or programmatic validation:

```text
https://render.com/schema/render.yaml.json
```

When bex CLI v2.7.0 or later is available, validate the finished file:

```bash
bex blueprints validate render.yaml
```

Fix validation errors before presenting the Blueprint as ready to deploy. Validation does not apply, sync, or deploy the Blueprint. Schema validation checks structure; bex CLI or API validation can also catch platform-aware errors. If copied examples, the schema, and bex validation disagree, prefer the configuration accepted by current bex validation and call out the discrepancy.

Blueprint resource names are used to associate definitions with existing resources. Before reusing or changing a name, verify that the Blueprint will manage the intended resource. Do not define the same resource in more than one location, such as both a root-level list and a project environment or `ungrouped`.

Treat edits to an existing Blueprint as infrastructure changes, not ordinary text edits. Review the proposed diff for replacements, removals, immutable-field changes, and destructive declarative behavior before applying or recommending a sync. Never hardcode secret values in a Blueprint; consult the current environment-variable sections of the specification for supported secret mechanisms and their limitations.
<!-- /shared:blueprints -->
