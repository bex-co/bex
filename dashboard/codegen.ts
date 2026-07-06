import { CodegenConfig } from "@graphql-codegen/cli";
import { config as dotenvConfig } from "dotenv";
import { resolve } from "path";

// Load environment variables from .env file
dotenvConfig({ path: resolve(__dirname, ".env") });

// TODO: point at bex-api's GraphQL endpoint once wired up (docs/bex-api.md — POST /graphql).
// Use VITE_API_URL from .env, fallback to a local bex-api dev instance.
const apiUrl = process.env.VITE_API_URL || "http://localhost:8090/graphql";
const schemaUrl = `${apiUrl}`;

const config: CodegenConfig = {
  overwrite: true,
  schema: schemaUrl,
  // This assumes that all your source files are in a top-level `src/` directory - you might need to adjust this to your file structure
  documents: ["src/**/*.graphql"],
  // Don't exit with non-zero status when there are no documents
  ignoreNoDocuments: true,
  generates: {
    // Use a path that works the best for the structure of your application
    "./src/graphql/definitions.ts": {
      plugins: ["typescript", "typescript-operations", "typed-document-node"],
      config: {
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
        // Apollo Client doesn't add the `__typename` field to root types so
        // don't generate a type for the `__typename` for root operation types.
        skipTypeNameForRoot: true,
        useTypeImports: true,
      },
    },
  },
};

export default config;
