import fs from "fs";
import path from "path";
import ts from "typescript";
import { Kind, parse, visit } from "graphql";
import { MOBILE_SAFE_ACTIONS } from "../components/safe-action/registry";

const srcRoot = path.resolve(process.cwd(), "src");
const routeRoot = path.resolve(process.cwd(), "app");
const productionRoots = [srcRoot, routeRoot];
const sourceExtensions = [".ts", ".tsx", ".js", ".jsx"] as const;

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

const routeFiles = sourceFiles(routeRoot, sourceExtensions)
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
  "invite.tsx",
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
  "MobilePatchSingleEnvVar",
  "MobileRunCronJob",
  "MobileCancelCronRun",
  "MobileRegisterNotificationDeviceSubscription",
  "MobileUnregisterNotificationDeviceSubscription",
  "MobileMarkPushNotificationRead",
  "MobileAcceptWorkspaceInvite",
  "MobileCreateAgentSession",
  "MobileCancelAgentSession",
]);
const allowedMutationDocuments = new Set(
  [...allowedMutationNames].map((name) => `${name}Document`),
);

// name -> operation kind | sorted variables | sorted selected field names.
// Any new read, write, argument surface, or selected field needs an explicit
// supervision-scope review instead of entering mobile through codegen silently.
const allowedGraphqlOperations: Record<string, string> = {
  MobileAcceptWorkspaceInvite:
    "mutation|token|acceptWorkspaceInvite,role,workspaceId,workspaceName",
  MobileAgentRepos: "query|ownerId|defaultBranch,fullName,private,repos",
  MobileAgentSession:
    "query|id|agentSession,branch,canceledAt,changedFiles,commandLog,commits,createdAt,deliveryMode,evidence,failureReason,headSha,id,outputTail,phase,prNumber,prUrl,repo,status,testOutput,truncated,turns,updatedAt",
  MobileAgentSessionCapabilities:
    "query|ownerId|accountLogin,agentSessionCapabilities,agents,connected,enabled,github,id,installUrl,label,modelKeyReady,ready",
  MobileAgentSessions:
    "query|ownerId|agentSessions,branch,createdAt,failureReason,id,phase,prNumber,prUrl,repo,status,updatedAt",
  MobileCancelAgentSession:
    "mutation|id|cancelAgentSession,canceledAt,id,phase,status",
  MobileCreateAgentSession:
    "mutation|agentConfig,branch,ownerId,repo|branch,createAgentSession,createdAt,id,phase,repo,status",
  MobileCancelCronRun:
    "mutation|runId,serviceId|cancelCronJobRun,finishedAt,id,startedAt,status",
  MobileCancelDeploy:
    "mutation|deployId,serviceId|cancelDeploy,finishedAt,id,serviceId,status,updatedAt",
  MobileCronRuns:
    "query|cursor,limit,serviceId|cronJobRuns,finishedAt,id,startedAt,status",
  MobileDeployHistory:
    "query|cursor,limit,serviceId|commitCreatedAt,commitId,commitMessage,createdAt,deploys,failureReason,finishedAt,id,image,preDeployStatus,rollbackOf,serviceId,startedAt,status,trigger,updatedAt",
  MobileEnvVarKeys: "query|serviceId|envVarKeys,id,key,revision,service",
  MobileKeyValueInsights:
    "query|id|datastoreMetrics,field,labels,time,unit,value,values",
  MobileKeyValueLifecycle:
    "query|id|id,keyValue,name,plan,region,status,suspended,updatedAt,version",
  MobileMarkPushNotificationRead: "mutation|id|markPushNotificationRead",
  MobileMetricSnapshot:
    "query|name,resourceId|field,labels,metrics,time,unit,value,values",
  MobileNotificationDeviceSubscriptions:
    "query||createdAt,deviceId,lastRegisteredAt,notificationDeviceSubscriptions,platform,preferenceRef,provider,pushNotificationsAvailable,updatedAt",
  MobileNotificationInbox:
    "query|limit|body,deepLink,event,id,notificationInbox,occurredAt,readAt,title,unreadPushNotificationCount",
  MobilePatchSingleEnvVar:
    "mutation|key,revision,serviceId,value|envVarKeys,patchServiceEnvironment,rolledOut",
  MobilePostgresCapacity:
    "query|id|datastoreMetrics,field,labels,time,unit,value,values",
  MobilePostgresLifecycle:
    "query|id|database,id,name,plan,region,status,suspended,updatedAt,version",
  MobilePostgresProcesses:
    "query|id|databaseProcesses,durationSeconds,state,waitEventType",
  MobilePostgresSizes:
    "query|id|database,databaseSizes,name,schema,sizePretty,tables",
  MobilePostgresTableScans:
    "query|id|databaseTableScans,deadRows,indexScans,name,schema,seqScans",
  MobileRegisterNotificationDeviceSubscription:
    "mutation|deviceId,platform,provider,token|createdAt,deviceId,lastRegisteredAt,platform,preferenceRef,provider,registerNotificationDeviceSubscription,updatedAt",
  MobileResourceStatus:
    "query|ownerId|databaseIds,databases,displayName,id,keyValueIds,keyValues,latestDeployId,name,phase,projectId,projects,runtime,serviceIds,services,status,suspended,type,updatedAt,version",
  MobileRestartPostgres:
    "mutation|id|id,restartDatabase,status,suspended,updatedAt",
  MobileRestartService:
    "mutation|serviceId|createdAt,id,restartServer,serviceId,status",
  MobileResumeKeyValue:
    "mutation|id|id,resumeKeyValue,status,suspended,updatedAt",
  MobileResumePostgres:
    "mutation|id|id,resumeDatabase,status,suspended,updatedAt",
  MobileResumeService: "mutation|id|id,phase,resumeService,suspended,updatedAt",
  MobileRevealEnvVar:
    "query|key,serviceId|envVar,id,key,revision,service,value",
  MobileRollbackService:
    "mutation|deployId,serviceId|createdAt,id,rollbackOf,rollbackService,serviceId,status,trigger",
  MobileRunCronJob: "mutation|id|finishedAt,id,runCronJob,startedAt,status",
  MobileServiceEvents:
    "query|cursor,limit,serviceId|actor,branchFrom,branchTo,commitId,commitMessage,commitUrl,cursor,deployId,deployStatus,details,finishedAt,fromCount,id,image,instanceId,preDeployStatus,reasonCode,serviceEvents,startedAt,status,timestamp,toCount,triggeredByUser,type",
  MobileServiceSupervision:
    "query|id|displayName,id,latestDeployId,name,phase,region,replicas,revision,runtime,service,suspended,type,updatedAt",
  MobileSuspendKeyValue:
    "mutation|confirm,id|id,status,suspendKeyValue,suspended,updatedAt",
  MobileSuspendPostgres:
    "mutation|confirm,id|id,status,suspendDatabase,suspended,updatedAt",
  MobileSuspendService:
    "mutation|confirm,id|id,phase,suspendService,suspended,updatedAt",
  MobileTriggerDeploy:
    "mutation|serviceId|createdAt,id,serviceId,status,trigger,triggerDeploy",
  MobileUnregisterNotificationDeviceSubscription:
    "mutation|deviceId|unregisterNotificationDeviceSubscription",
  MobileUsageGlance:
    "query|ownerId|coverage,degradedSources,kind,period,rows,services,state,through,total,usage",
  MobileWorkspaces: "query||createdAt,id,name,plan,role,workspaces",
};

const allowedDirectWriteMethods = new Set([
  "src/features/auth/expo-oauth-transport.ts:POST",
]);

// Independent deny floor: updating the positive inventory must not silently
// admit desktop/admin/destructive controls into the native app.
const forbiddenRouteSegment =
  /(?:^|[/.-])(?:new|settings|billing|invoice|checkout|portal|shell|backup|recovery|failover|parameter|allow-?list|env-?group|secret-?file)(?:[/.-]|$)/i;
const forbiddenMutationControl =
  /(?:delete|remove|destroy|failover|restore|recover|upgrade|create(?:service|database|keyvalue|workspace|project)|change.*plan|set.*(?:parameter|allowlist)|envgroup|secretfile|billing|checkout|portal|payment|invoice|tax)/i;
const forbiddenSensitiveFields = new Set([
  "billing",
  "checkout",
  "connectionInfo",
  "connectionString",
  "databaseConnectionInfo",
  "databaseTopQueries",
  "databaseUsers",
  "estimatedCost",
  "externalConnectionString",
  "internalConnectionString",
  "invoices",
  "ipAllowList",
  "parameterOverrides",
  "password",
  "paymentMethod",
  "portal",
  "query",
  "recoveryInfo",
  "sql",
  "tax",
]);

interface ProductWriteInventory {
  actionIds: string[];
  mutationDocuments: string[];
  directWriteMethods: string[];
}

function scriptKind(file: string): ts.ScriptKind {
  if (file.endsWith(".tsx")) return ts.ScriptKind.TSX;
  if (file.endsWith(".jsx")) return ts.ScriptKind.JSX;
  if (file.endsWith(".js")) return ts.ScriptKind.JS;
  return ts.ScriptKind.TS;
}

function productWriteInventory(): ProductWriteInventory {
  const inventory: ProductWriteInventory = {
    actionIds: [],
    mutationDocuments: [],
    directWriteMethods: [],
  };
  for (const root of productionRoots) {
    for (const file of sourceFiles(root, sourceExtensions)) {
      const source = ts.createSourceFile(
        file,
        fs.readFileSync(file, "utf8"),
        ts.ScriptTarget.Latest,
        true,
        scriptKind(file),
      );
      const inspect = (node: ts.Node) => {
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
        ts.forEachChild(node, inspect);
      };
      inspect(source);
    }
  }
  return inventory;
}

function graphqlOperationInventory(): Record<string, string> {
  const operations: Record<string, string> = {};
  for (const root of productionRoots) {
    for (const file of sourceFiles(root, [".graphql"])) {
      for (const definition of parse(fs.readFileSync(file, "utf8"))
        .definitions) {
        if (definition.kind !== Kind.OPERATION_DEFINITION) continue;
        const name = definition.name?.value ?? "<anonymous-operation>";
        if (operations[name])
          throw new Error(`duplicate GraphQL operation: ${name}`);
        const fields: string[] = [];
        visit(definition, {
          Field: (node) => void fields.push(node.name.value),
        });
        const variables = (definition.variableDefinitions ?? [])
          .map((variable) => variable.variable.name.value)
          .sort();
        operations[name] = `${definition.operation}|${variables.join(",")}|${[
          ...new Set(fields),
        ]
          .sort()
          .join(",")}`;
      }
    }
  }
  return Object.fromEntries(Object.entries(operations).sort());
}

function forbiddenRoutes(routes: readonly string[]): string[] {
  return routes.filter((route) => forbiddenRouteSegment.test(route)).sort();
}

function forbiddenGraphqlControls(sources: readonly string[]): string[] {
  const found = new Set<string>();
  for (const source of sources) {
    for (const definition of parse(source).definitions) {
      if (definition.kind !== Kind.OPERATION_DEFINITION) continue;
      if (definition.operation === "mutation") {
        for (const selection of definition.selectionSet.selections) {
          if (
            selection.kind === Kind.FIELD &&
            forbiddenMutationControl.test(selection.name.value)
          ) {
            found.add(`mutation:${selection.name.value}`);
          }
        }
      }
      visit(definition, {
        Field: (node) => {
          if (forbiddenSensitiveFields.has(node.name.value)) {
            found.add(`field:${node.name.value}`);
          }
        },
      });
    }
  }
  return [...found].sort();
}

function graphqlSources(): string[] {
  return productionRoots.flatMap((root) =>
    sourceFiles(root, [".graphql"]).map((file) =>
      fs.readFileSync(file, "utf8"),
    ),
  );
}

describe("ADR048 mobile scope", () => {
  it("exposes only supervision and approved platform entry routes", () => {
    expect(routeFiles).toEqual(allowedRoutes);
  });

  it("keeps credentials out of ordinary storage and diagnostics", () => {
    const authRoot = path.resolve(process.cwd(), "src/features/auth");
    const authText = sourceFiles(authRoot, sourceExtensions)
      .map((file) => fs.readFileSync(file, "utf8"))
      .join("\n");
    expect(authText).not.toContain("AsyncStorage");
    expect(authText).not.toContain("console.");
    expect(authText).not.toContain("WebView");
    expect(authText).not.toContain("clientSecret");
  });

  it("registers only the exact safe-action and mutation inventory", () => {
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
    expect(
      inventory.directWriteMethods.filter(
        (write) => !allowedDirectWriteMethods.has(write),
      ),
    ).toEqual([]);
    expect([...new Set(inventory.directWriteMethods)].sort()).toEqual(
      [...allowedDirectWriteMethods].sort(),
    );
  });

  it("allowlists every GraphQL operation, variable, and selected field", () => {
    const operations = graphqlOperationInventory();
    expect(operations).toEqual(allowedGraphqlOperations);
    expect(
      Object.entries(operations)
        .filter(([, signature]) => signature.startsWith("mutation|"))
        .map(([name]) => name)
        .sort(),
    ).toEqual([...allowedMutationNames].sort());
  });

  it("independently denies desktop, admin, and destructive routes", () => {
    expect(forbiddenRoutes(routeFiles)).toEqual([]);
    expect(
      forbiddenRoutes([
        "(app)/services/[serviceId].tsx",
        "(app)/billing.tsx",
        "(app)/databases/[databaseId]/recovery.tsx",
        "(app)/services/new.tsx",
      ]),
    ).toEqual([
      "(app)/billing.tsx",
      "(app)/databases/[databaseId]/recovery.tsx",
      "(app)/services/new.tsx",
    ]);
  });

  it("independently denies sensitive and destructive GraphQL controls", () => {
    expect(forbiddenGraphqlControls(graphqlSources())).toEqual([]);
    expect(
      forbiddenGraphqlControls([
        'mutation MobileDeleteService { deleteService(id: "srv-one") }',
        'query MobileConnectionLeak { databaseConnectionInfo(id: "dpg-one") { password } }',
      ]),
    ).toEqual([
      "field:databaseConnectionInfo",
      "field:password",
      "mutation:deleteService",
    ]);
  });
});
