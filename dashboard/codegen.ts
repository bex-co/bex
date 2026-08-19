import { CodegenConfig } from "@graphql-codegen/cli";
import { config as dotenvConfig } from "dotenv";
import { resolve } from "path";

// Load environment variables from .env file
dotenvConfig({ path: resolve(__dirname, ".env") });

// Points at bex-api's GraphQL endpoint (docs/ADR006-bex-api.md — POST /graphql):
// VITE_API_URL from .env, falling back to a local bex-api dev instance.
const apiUrl = process.env.VITE_API_URL || "http://localhost:8090/graphql";

// Every bex-api route requires a real credential (docs/ADR012-auth.md) — introspection
// is no exception. Export CODEGEN_SESSION_TOKEN (an Ory session token — the
// dashboard's own auth mechanism; log in, or mint one via Kratos's registration
// API) before running `yarn codegen` so it can reach the schema.
const sessionToken = process.env.CODEGEN_SESSION_TOKEN;

// SCHEMA_JSON, when set, points at a GraphQL introspection JSON file so codegen
// can regenerate offline without a live bex-api — dump it from the backend with
// `SCHEMA_DUMP_PATH=... go test ./internal/api -run TestDumpGraphQLSchema`
// (internal/api/schema_dump_test.go). Unset => the live-endpoint path below.
const schemaJSON = process.env.SCHEMA_JSON;

const typeConfig = {
  avoidOptionals: {
    // Use `null` for nullable fields instead of optionals
    field: true,
    // Allow nullable input fields to remain unspecified
    inputValue: false,
  },
  // Use `unknown` instead of `any` for unconfigured scalars
  defaultScalarType: "unknown",
  // Map DateTimeISO to string type
  scalars: {
    DateTimeISO: "string",
    JSONObject: "Record<string, unknown>",
  },
  // Apollo Client always includes `__typename` fields
  nonOptionalTypename: true,
  // Apollo Client doesn't add `__typename` to root operation types.
  skipTypeNameForRoot: true,
  useTypeImports: true,
} as const;

const config: CodegenConfig = {
  overwrite: true,
  // The `typescript` (full-schema) plugin currently throws inside the
  // generator — "Cannot read properties of undefined (reading 'find')" — for
  // *any* schema, including `type Query { hello: String }`, so it is a broken
  // plugin install rather than anything about bex's schema. Codegen aborts all
  // outputs on any error, which made even the healthy `typescript-operations`
  // target unregenerable. Allowing partial output keeps definitions.ts (what
  // every operation imports) refreshable; schema-types.ts simply retains its
  // previous contents and needs a hand-edit for new types until the plugin is
  // repaired. Re-check on the next @graphql-codegen bump and drop this.
  allowPartialOutputs: true,
  schema: schemaJSON
    ? schemaJSON
    : sessionToken
      ? [{ [apiUrl]: { headers: { "X-Session-Token": sessionToken } } }]
      : apiUrl,
  // This assumes that all your source files are in a top-level `src/` directory - you might need to adjust this to your file structure
  documents: ["src/**/*.graphql"],
  // Don't exit with non-zero status when there are no documents
  ignoreNoDocuments: true,
  generates: {
    // Keep the complete schema surface available to the few dashboard helpers
    // that intentionally consume schema types rather than operation results.
    "./src/graphql/schema-types.ts": {
      plugins: ["typescript"],
      config: typeConfig,
    },
    // GraphQL Codegen v6's operations plugin owns its utility/input types. Keep
    // it separate from the full schema output, then re-export the schema types
    // so existing dashboard imports continue to have one stable entry point.
    "./src/graphql/definitions.ts": {
      plugins: [
        {
          add: {
            content: 'export * from "./schema-types";',
          },
        },
        "typescript-operations",
        "typed-document-node",
      ],
      config: {
        ...typeConfig,
        importSchemaTypesFrom: "./src/graphql/schema-types.ts",
        namespacedImportName: "Types",
      },
    },
  },
};

export default config;
