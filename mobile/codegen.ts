import type { CodegenConfig } from "@graphql-codegen/cli";
import { config as dotenvConfig } from "dotenv";

dotenvConfig({ path: ".env" });

const apiUrl = `${process.env.EXPO_PUBLIC_BEX_API_URL ?? "http://localhost:8090"}/graphql`;
const schemaJSON = process.env.SCHEMA_JSON;
const token = process.env.CODEGEN_BEARER_TOKEN;

const config: CodegenConfig = {
  overwrite: true,
  schema: schemaJSON
    ? schemaJSON
    : token
      ? [{ [apiUrl]: { headers: { Authorization: `Bearer ${token}` } } }]
      : apiUrl,
  documents: ["src/**/*.graphql"],
  generates: {
    "src/generated-graphql/index.ts": {
      plugins: ["typescript", "typescript-operations", "typed-document-node"],
      config: {
        avoidOptionals: { field: true, inputValue: false },
        defaultScalarType: "unknown",
        scalars: {
          DateTimeISO: "string",
          JSONObject: "Record<string, unknown>",
        },
        nonOptionalTypename: true,
        skipTypeNameForRoot: true,
        useTypeImports: true,
      },
    },
  },
};

export default config;
