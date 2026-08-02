import fs from "fs";
import path from "path";
import ts from "typescript";
import { Kind, parse } from "graphql";
import { MOBILE_SAFE_ACTIONS } from "../components/safe-action/registry";

const routeRoot = path.resolve(process.cwd(), "app");

function sourceFiles(root: string, extensions: readonly string[]): string[] {
  return fs
    .readdirSync(root, { recursive: true })
    .map(String)
    .filter((entry) =>
      extensions.some((extension) => entry.endsWith(extension)),
    )
    .filter((entry) => !entry.includes("__tests__"))
    .filter((entry) => !entry.includes("generated-graphql"))
    .map((entry) => path.join(root, entry));
}

const routeFiles = sourceFiles(routeRoot, [".tsx"])
  .map((file) => path.relative(routeRoot, file))
  .sort();

const allowedRoutes = [
  "(app)/_layout.tsx",
  "(app)/activity.tsx",
  "(app)/databases/[databaseId].tsx",
  "(app)/index.tsx",
  "(app)/key-values/[keyValueId].tsx",
  "(app)/notifications.tsx",
  "(app)/services/[serviceId].tsx",
  "(app)/services/[serviceId]/logs.tsx",
  "(app)/sessions.tsx",
  "(app)/sessions/[sessionId].tsx",
  "+not-found.tsx",
  "_layout.tsx",
  "index.tsx",
  "oauth2redirect.tsx",
  "sign-in.tsx",
].sort();

const allowedMutationNames = new Set([
  "MobileTriggerDeploy",
  "MobileCancelDeploy",
  "MobileRollbackService",
  "MobileRestartService",
  "MobileSuspendService",
  "MobileResumeService",
  "MobileRestartPostgres",
  "MobileSuspendPostgres",
  "MobileResumePostgres",
  "MobileSuspendKeyValue",
  "MobileResumeKeyValue",
  "MobileRegisterNotificationDeviceSubscription",
  "MobileUnregisterNotificationDeviceSubscription",
  "MobileMarkPushNotificationRead",
]);
const allowedMutationDocuments = new Set(
  [...allowedMutationNames].map((name) => `${name}Document`),
);

interface ProductWriteInventory {
  actionIds: string[];
  mutationDocuments: string[];
  directWriteMethods: string[];
}

function productWriteInventory(): ProductWriteInventory {
  const inventory: ProductWriteInventory = {
    actionIds: [],
    mutationDocuments: [],
    directWriteMethods: [],
  };
  const roots = [path.resolve(process.cwd(), "src/features"), routeRoot];
  for (const root of roots) {
    for (const file of sourceFiles(root, [".ts", ".tsx"])) {
      if (file.includes(`${path.sep}features${path.sep}auth${path.sep}`))
        continue;
      const source = ts.createSourceFile(
        file,
        fs.readFileSync(file, "utf8"),
        ts.ScriptTarget.Latest,
        true,
        file.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
      );
      const visit = (node: ts.Node) => {
        if (
          ts.isCallExpression(node) &&
          ts.isIdentifier(node.expression) &&
          node.expression.text === "defineSafeAction"
        ) {
          const id = node.arguments[0];
          inventory.actionIds.push(
            id && ts.isStringLiteral(id) ? id.text : "<non-literal-action>",
          );
        }
        if (
          ts.isCallExpression(node) &&
          ts.isIdentifier(node.expression) &&
          node.expression.text === "useMutation"
        ) {
          const document = node.arguments[0];
          inventory.mutationDocuments.push(
            document && ts.isIdentifier(document)
              ? document.text
              : "<dynamic-mutation>",
          );
        }
        if (
          ts.isPropertyAssignment(node) &&
          ((ts.isIdentifier(node.name) && node.name.text === "mutation") ||
            (ts.isStringLiteral(node.name) && node.name.text === "mutation"))
        ) {
          inventory.mutationDocuments.push(
            ts.isIdentifier(node.initializer)
              ? node.initializer.text
              : "<dynamic-mutation>",
          );
        }
        if (
          ts.isPropertyAssignment(node) &&
          ((ts.isIdentifier(node.name) && node.name.text === "method") ||
            (ts.isStringLiteral(node.name) && node.name.text === "method")) &&
          ts.isStringLiteral(node.initializer) &&
          ["POST", "PUT", "PATCH", "DELETE"].includes(
            node.initializer.text.toUpperCase(),
          )
        ) {
          inventory.directWriteMethods.push(
            `${path.relative(process.cwd(), file)}:${node.initializer.text.toUpperCase()}`,
          );
        }
        ts.forEachChild(node, visit);
      };
      visit(source);
    }
  }
  return inventory;
}

function graphqlMutations(): string[] {
  const featureRoot = path.resolve(process.cwd(), "src/features");
  return sourceFiles(featureRoot, [".graphql"]).flatMap((file) =>
    parse(fs.readFileSync(file, "utf8")).definitions.flatMap((definition) =>
      definition.kind === Kind.OPERATION_DEFINITION &&
      definition.operation === "mutation"
        ? [definition.name?.value ?? "<anonymous-mutation>"]
        : [],
    ),
  );
}

describe("ADR048 mobile scope", () => {
  it("exposes only the supervision tabs at the app root", () => {
    expect(routeFiles).toEqual(allowedRoutes);
  });

  it("keeps credentials out of ordinary storage and diagnostics", () => {
    const authRoot = path.resolve(process.cwd(), "src/features/auth");
    const authText = fs
      .readdirSync(authRoot, { recursive: true })
      .filter(
        (entry) =>
          String(entry).endsWith(".ts") || String(entry).endsWith(".tsx"),
      )
      .filter((entry) => !String(entry).includes("__tests__"))
      .map((entry) =>
        fs.readFileSync(path.join(authRoot, String(entry)), "utf8"),
      )
      .join("\n");
    expect(authText).not.toContain("AsyncStorage");
    expect(authText).not.toContain("console.");
    expect(authText).not.toContain("WebView");
    expect(authText).not.toContain("clientSecret");
  });

  it("registers only the exact safe action and mutation inventory", () => {
    const inventory = productWriteInventory();
    const allowedActionIds = new Set<string>(MOBILE_SAFE_ACTIONS);
    expect(
      inventory.actionIds.filter((id) => !allowedActionIds.has(id)),
    ).toEqual([]);
    expect([...new Set(inventory.actionIds)].sort()).toEqual(
      [...MOBILE_SAFE_ACTIONS].sort(),
    );
    expect(
      inventory.mutationDocuments.filter(
        (document) => !allowedMutationDocuments.has(document),
      ),
    ).toEqual([]);
    expect([...new Set(graphqlMutations())].sort()).toEqual(
      [...allowedMutationNames].sort(),
    );
    // Product writes must pass through typed GraphQL + defineSafeAction. Auth's
    // OAuth transport is excluded above; harmless prose/tests are never scanned.
    expect(inventory.directWriteMethods).toEqual([]);
  });
});
