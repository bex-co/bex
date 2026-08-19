export type Maybe<T> = T | null;
export type InputMaybe<T> = Maybe<T>;
/** All built-in and custom scalars, mapped to their actual values */
export type Scalars = {
  ID: { input: string; output: string };
  String: { input: string; output: string };
  Boolean: { input: boolean; output: boolean };
  Int: { input: number; output: number };
  Float: { input: number; output: number };
};

export type AcceptedWorkspaceInvite = {
  __typename: "AcceptedWorkspaceInvite";
  authorizationPending: Maybe<Scalars["Boolean"]["output"]>;
  role: Maybe<Scalars["String"]["output"]>;
  workspaceId: Maybe<Scalars["String"]["output"]>;
  workspaceName: Maybe<Scalars["String"]["output"]>;
};

export type AgentSession = {
  __typename: "AgentSession";
  agentConfig: AgentSessionConfig;
  archivedAt: Maybe<Scalars["String"]["output"]>;
  branch: Scalars["String"]["output"];
  canceledAt: Maybe<Scalars["String"]["output"]>;
  createdAt: Scalars["String"]["output"];
  deliveryMode: Maybe<Scalars["String"]["output"]>;
  evidence: Maybe<AgentSessionEvidence>;
  expiresAt: Maybe<Scalars["String"]["output"]>;
  failureReason: Maybe<Scalars["String"]["output"]>;
  headSha: Maybe<Scalars["String"]["output"]>;
  hibernatedAt: Maybe<Scalars["String"]["output"]>;
  id: Scalars["String"]["output"];
  ownerId: Scalars["String"]["output"];
  phase: Scalars["String"]["output"];
  pinned: Maybe<Scalars["Boolean"]["output"]>;
  prNumber: Maybe<Scalars["Int"]["output"]>;
  prUrl: Maybe<Scalars["String"]["output"]>;
  repo: Scalars["String"]["output"];
  retainUntil: Maybe<Scalars["String"]["output"]>;
  sandboxId: Maybe<Scalars["String"]["output"]>;
  snapshotBytes: Maybe<Scalars["Float"]["output"]>;
  sshAddress: Maybe<Scalars["String"]["output"]>;
  status: Scalars["String"]["output"];
  streamUrl: Maybe<Scalars["String"]["output"]>;
  ticket: Maybe<Scalars["String"]["output"]>;
  turns: Maybe<Scalars["Int"]["output"]>;
  updatedAt: Scalars["String"]["output"];
  url: Maybe<Scalars["String"]["output"]>;
};

export type AgentSessionCapabilities = {
  __typename: "AgentSessionCapabilities";
  agents: Array<AgentSessionProfile>;
  enabled: Scalars["Boolean"]["output"];
  github: AgentSessionGitHubReadiness;
  modelKeyReady: Scalars["Boolean"]["output"];
  ready: Scalars["Boolean"]["output"];
};

export type AgentSessionConfig = {
  __typename: "AgentSessionConfig";
  agent: Scalars["String"]["output"];
  model: Maybe<Scalars["String"]["output"]>;
  modelEndpoint: Maybe<Scalars["String"]["output"]>;
  task: Scalars["String"]["output"];
  template: Maybe<Scalars["String"]["output"]>;
};

export type AgentSessionConfigInput = {
  agent: Scalars["String"]["input"];
  model?: InputMaybe<Scalars["String"]["input"]>;
  modelEndpoint?: InputMaybe<Scalars["String"]["input"]>;
  task: Scalars["String"]["input"];
  template?: InputMaybe<Scalars["String"]["input"]>;
};

export type AgentSessionEvidence = {
  __typename: "AgentSessionEvidence";
  changedFiles: Maybe<Array<Scalars["String"]["output"]>>;
  commandLog: Maybe<Array<Scalars["String"]["output"]>>;
  commits: Maybe<Scalars["Int"]["output"]>;
  outputTail: Maybe<Scalars["String"]["output"]>;
  testOutput: Maybe<Array<Scalars["String"]["output"]>>;
  truncated: Maybe<Scalars["Boolean"]["output"]>;
};

export type AgentSessionGitHubReadiness = {
  __typename: "AgentSessionGitHubReadiness";
  accountLogin: Maybe<Scalars["String"]["output"]>;
  connected: Scalars["Boolean"]["output"];
  installUrl: Maybe<Scalars["String"]["output"]>;
};

export type AgentSessionProfile = {
  __typename: "AgentSessionProfile";
  id: Scalars["String"]["output"];
  label: Scalars["String"]["output"];
};

export type AgentSessionTranscriptPage = {
  __typename: "AgentSessionTranscriptPage";
  nextAfterSeq: Scalars["Int"]["output"];
  parts: Array<AgentSessionTranscriptPart>;
  turns: Array<AgentSessionTranscriptTurn>;
};

export type AgentSessionTranscriptPart = {
  __typename: "AgentSessionTranscriptPart";
  createdAt: Scalars["String"]["output"];
  part: Scalars["String"]["output"];
  partIndex: Scalars["Int"]["output"];
  seq: Scalars["Int"]["output"];
  turn: Scalars["Int"]["output"];
};

export type AgentSessionTranscriptTurn = {
  __typename: "AgentSessionTranscriptTurn";
  completedAt: Maybe<Scalars["String"]["output"]>;
  createdAt: Scalars["String"]["output"];
  deliveryMode: Scalars["String"]["output"];
  prompt: Scalars["String"]["output"];
  transcriptComplete: Scalars["Boolean"]["output"];
  transcriptTruncated: Scalars["Boolean"]["output"];
  truncationReason: Scalars["String"]["output"];
  turn: Scalars["Int"]["output"];
};

export type ApiKey = {
  __typename: "ApiKey";
  createdAt: Maybe<Scalars["String"]["output"]>;
  createdBy: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  lastUsedAt: Maybe<Scalars["String"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
  secret: Maybe<Scalars["String"]["output"]>;
};

export type AuditLog = {
  __typename: "AuditLog";
  action: Maybe<Scalars["String"]["output"]>;
  actor: Maybe<Scalars["String"]["output"]>;
  actorMethod: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  metadata: Maybe<AuditLogMetadata>;
  oauthAudience: Maybe<Scalars["String"]["output"]>;
  oauthClientId: Maybe<Scalars["String"]["output"]>;
  oauthScopes: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  relation: Maybe<Scalars["String"]["output"]>;
  resource: Maybe<Scalars["String"]["output"]>;
  status: Maybe<Scalars["String"]["output"]>;
  targetName: Maybe<Scalars["String"]["output"]>;
  timestamp: Maybe<Scalars["String"]["output"]>;
};

export type AuditLogMetadata = {
  __typename: "AuditLogMetadata";
  to: Maybe<Scalars["Boolean"]["output"]>;
};

export type Autoscaling = {
  __typename: "Autoscaling";
  enabled: Maybe<Scalars["Boolean"]["output"]>;
  maxInstances: Maybe<Scalars["Int"]["output"]>;
  minInstances: Maybe<Scalars["Int"]["output"]>;
  targetCPUPercent: Maybe<Scalars["Int"]["output"]>;
  targetMemoryPercent: Maybe<Scalars["Int"]["output"]>;
};

export type Billing = {
  __typename: "Billing";
  credits: Maybe<BillingCredits>;
  currentCost: Maybe<BillingAmount>;
  invoices: Maybe<Array<Maybe<BillingInvoice>>>;
};

export type BillingAmount = {
  __typename: "BillingAmount";
  amountUsd: Maybe<Scalars["String"]["output"]>;
  currency: Maybe<Scalars["String"]["output"]>;
  periodEnd: Maybe<Scalars["String"]["output"]>;
  periodStart: Maybe<Scalars["String"]["output"]>;
};

export type BillingCreditGrant = {
  __typename: "BillingCreditGrant";
  expiresAt: Maybe<Scalars["String"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
  remainingUsd: Maybe<Scalars["String"]["output"]>;
};

export type BillingCredits = {
  __typename: "BillingCredits";
  availableUsd: Maybe<Scalars["String"]["output"]>;
  currency: Maybe<Scalars["String"]["output"]>;
  grants: Maybe<Array<Maybe<BillingCreditGrant>>>;
};

export type BillingHostedSession = {
  __typename: "BillingHostedSession";
  expiresAt: Maybe<Scalars["String"]["output"]>;
  url: Maybe<Scalars["String"]["output"]>;
};

export type BillingInvoice = {
  __typename: "BillingInvoice";
  amountUsd: Maybe<Scalars["String"]["output"]>;
  currency: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  periodEnd: Maybe<Scalars["String"]["output"]>;
  periodStart: Maybe<Scalars["String"]["output"]>;
  status: Maybe<Scalars["String"]["output"]>;
};

export type BillingLifecycle = {
  __typename: "BillingLifecycle";
  allowedActions: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  enforcementOwned: Maybe<Scalars["Boolean"]["output"]>;
  graceDeadline: Maybe<Scalars["String"]["output"]>;
  reason: Maybe<Scalars["String"]["output"]>;
  recoveryPending: Maybe<Scalars["Boolean"]["output"]>;
  status: Maybe<Scalars["String"]["output"]>;
  updatedAt: Maybe<Scalars["String"]["output"]>;
};

export type BillingTaxReadiness = {
  __typename: "BillingTaxReadiness";
  configured: Maybe<Scalars["Boolean"]["output"]>;
  enabled: Maybe<Scalars["Boolean"]["output"]>;
  productTaxCode: Maybe<Scalars["String"]["output"]>;
  reason: Maybe<Scalars["String"]["output"]>;
  registrationCount: Maybe<Scalars["Int"]["output"]>;
  taxBehavior: Maybe<Scalars["String"]["output"]>;
};

export type Blueprint = {
  __typename: "Blueprint";
  autoSync: Maybe<Scalars["Boolean"]["output"]>;
  branch: Maybe<Scalars["String"]["output"]>;
  createdAt: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  lastSync: Maybe<Scalars["String"]["output"]>;
  manifest: Maybe<Scalars["String"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
  path: Maybe<Scalars["String"]["output"]>;
  repo: Maybe<Scalars["String"]["output"]>;
  resources: Maybe<Array<Maybe<BlueprintResource>>>;
  status: Maybe<Scalars["String"]["output"]>;
  updatedAt: Maybe<Scalars["String"]["output"]>;
};

export type BlueprintEnvVarValueInput = {
  key: Scalars["String"]["input"];
  value: Scalars["String"]["input"];
};

export type BlueprintEstimatedPricing = {
  __typename: "BlueprintEstimatedPricing";
  lines: Maybe<Array<Maybe<BlueprintPricingLine>>>;
  totalUsd: Maybe<Scalars["String"]["output"]>;
  variable: Maybe<Array<Maybe<BlueprintVariableCost>>>;
};

export type BlueprintPlanAction = {
  __typename: "BlueprintPlanAction";
  changedFields: Maybe<Array<Maybe<BlueprintPlanFieldChange>>>;
  kind: Maybe<Scalars["String"]["output"]>;
  message: Maybe<Scalars["String"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
  operation: Maybe<Scalars["String"]["output"]>;
  resourceId: Maybe<Scalars["String"]["output"]>;
  sourcePath: Maybe<Scalars["String"]["output"]>;
};

export type BlueprintPlanFieldChange = {
  __typename: "BlueprintPlanFieldChange";
  path: Maybe<Scalars["String"]["output"]>;
};

export type BlueprintPreview = {
  __typename: "BlueprintPreview";
  commitId: Maybe<Scalars["String"]["output"]>;
  error: Maybe<Scalars["String"]["output"]>;
  found: Maybe<Scalars["Boolean"]["output"]>;
  manifest: Maybe<Scalars["String"]["output"]>;
  validation: Maybe<BlueprintValidation>;
  warning: Maybe<Scalars["String"]["output"]>;
};

export type BlueprintPricingLine = {
  __typename: "BlueprintPricingLine";
  instanceUsd: Maybe<Scalars["String"]["output"]>;
  monthlyUsd: Maybe<Scalars["String"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
  resourceKind: Maybe<Scalars["String"]["output"]>;
  storageGb: Maybe<Scalars["Int"]["output"]>;
  storageUsd: Maybe<Scalars["String"]["output"]>;
  tier: Maybe<Scalars["String"]["output"]>;
  tierLabel: Maybe<Scalars["String"]["output"]>;
};

export type BlueprintResource = {
  __typename: "BlueprintResource";
  id: Maybe<Scalars["String"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
  type: Maybe<Scalars["String"]["output"]>;
};

export type BlueprintSync = {
  __typename: "BlueprintSync";
  commitId: Maybe<Scalars["String"]["output"]>;
  completedAt: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  startedAt: Maybe<Scalars["String"]["output"]>;
  state: Maybe<Scalars["String"]["output"]>;
};

export type BlueprintValidation = {
  __typename: "BlueprintValidation";
  errorDetails: Maybe<Array<Maybe<BlueprintValidationError>>>;
  errors: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  estimatedPricing: Maybe<BlueprintEstimatedPricing>;
  plan: Maybe<BlueprintValidationPlan>;
  valid: Maybe<Scalars["Boolean"]["output"]>;
};

export type BlueprintValidationError = {
  __typename: "BlueprintValidationError";
  code: Maybe<Scalars["String"]["output"]>;
  column: Maybe<Scalars["Int"]["output"]>;
  error: Scalars["String"]["output"];
  line: Maybe<Scalars["Int"]["output"]>;
  path: Maybe<Scalars["String"]["output"]>;
};

export type BlueprintValidationPlan = {
  __typename: "BlueprintValidationPlan";
  actions: Maybe<Array<Maybe<BlueprintPlanAction>>>;
  databases: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  envGroups: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  keyValue: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  mode: Maybe<Scalars["String"]["output"]>;
  services: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  syncFalseVars: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  totalActions: Maybe<Scalars["Int"]["output"]>;
};

export type BlueprintVariableCost = {
  __typename: "BlueprintVariableCost";
  name: Maybe<Scalars["String"]["output"]>;
  reason: Maybe<Scalars["String"]["output"]>;
};

export type BuildFilter = {
  __typename: "BuildFilter";
  ignoredPaths: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  paths: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
};

export type BuildFilterInput = {
  ignoredPaths?: InputMaybe<Array<InputMaybe<Scalars["String"]["input"]>>>;
  paths?: InputMaybe<Array<InputMaybe<Scalars["String"]["input"]>>>;
};

export type ChargeLine = {
  __typename: "ChargeLine";
  costUsd: Maybe<Scalars["String"]["output"]>;
  kind: Maybe<Scalars["String"]["output"]>;
  quantity: Maybe<Scalars["String"]["output"]>;
  rateUsd: Maybe<Scalars["String"]["output"]>;
  tier: Maybe<Scalars["String"]["output"]>;
  unit: Maybe<Scalars["String"]["output"]>;
};

export type CronRun = {
  __typename: "CronRun";
  finishedAt: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
  startedAt: Maybe<Scalars["String"]["output"]>;
  status: Maybe<Scalars["String"]["output"]>;
};

export type CustomDomain = {
  __typename: "CustomDomain";
  dnsRecord: Maybe<DnsRecord>;
  domainType: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
  ownershipDnsRecord: Maybe<DnsRecord>;
  ownershipStatus: Maybe<Scalars["String"]["output"]>;
  redirectForName: Maybe<Scalars["String"]["output"]>;
  serverStatus: Maybe<Scalars["String"]["output"]>;
  verificationStatus: Maybe<Scalars["String"]["output"]>;
};

export type DnsRecord = {
  __typename: "DNSRecord";
  name: Maybe<Scalars["String"]["output"]>;
  type: Maybe<Scalars["String"]["output"]>;
  value: Maybe<Scalars["String"]["output"]>;
};

export type Database = {
  __typename: "Database";
  backupsEnabled: Maybe<Scalars["Boolean"]["output"]>;
  createdAt: Maybe<Scalars["String"]["output"]>;
  dashboardUrl: Maybe<Scalars["String"]["output"]>;
  databaseName: Maybe<Scalars["String"]["output"]>;
  databaseUser: Maybe<Scalars["String"]["output"]>;
  diskAutoscalingEnabled: Maybe<Scalars["Boolean"]["output"]>;
  diskSizeGB: Maybe<Scalars["Int"]["output"]>;
  environmentId: Maybe<Scalars["String"]["output"]>;
  externalHost: Maybe<Scalars["String"]["output"]>;
  highAvailabilityEnabled: Maybe<Scalars["Boolean"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  ipAllowList: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  ipAllowListEntries: Maybe<Array<Maybe<IpAllowListEntry>>>;
  name: Maybe<Scalars["String"]["output"]>;
  ownerId: Maybe<Scalars["String"]["output"]>;
  plan: Maybe<Scalars["String"]["output"]>;
  poolerEnabled: Maybe<Scalars["Boolean"]["output"]>;
  projectId: Maybe<Scalars["String"]["output"]>;
  public: Maybe<Scalars["Boolean"]["output"]>;
  readReplicas: Maybe<Array<Maybe<ReadReplicaView>>>;
  region: Maybe<Scalars["String"]["output"]>;
  status: Maybe<Scalars["String"]["output"]>;
  suspended: Maybe<Scalars["String"]["output"]>;
  updatedAt: Maybe<Scalars["String"]["output"]>;
  version: Maybe<Scalars["String"]["output"]>;
};

export type DatabaseBackup = {
  __typename: "DatabaseBackup";
  createdAt: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  status: Maybe<Scalars["String"]["output"]>;
};

export type DatabaseExport = {
  __typename: "DatabaseExport";
  createdAt: Maybe<Scalars["String"]["output"]>;
  expiresAt: Maybe<Scalars["String"]["output"]>;
  failureReason: Maybe<Scalars["String"]["output"]>;
  filename: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  status: Maybe<Scalars["String"]["output"]>;
  url: Maybe<Scalars["String"]["output"]>;
  urlExpiresAt: Maybe<Scalars["String"]["output"]>;
};

export type DatabaseInstanceType = {
  __typename: "DatabaseInstanceType";
  cpu: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  memory: Maybe<Scalars["String"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
  storageGB: Maybe<Scalars["Int"]["output"]>;
};

export type DatabaseLogEntry = {
  __typename: "DatabaseLogEntry";
  instance: Maybe<Scalars["String"]["output"]>;
  message: Maybe<Scalars["String"]["output"]>;
  timestamp: Maybe<Scalars["String"]["output"]>;
  type: Maybe<Scalars["String"]["output"]>;
};

export type DatabaseParameterOverride = {
  __typename: "DatabaseParameterOverride";
  description: Maybe<Scalars["String"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
  setting: Maybe<Scalars["String"]["output"]>;
  source: Maybe<Scalars["String"]["output"]>;
  unit: Maybe<Scalars["String"]["output"]>;
};

export type DatabaseProcess = {
  __typename: "DatabaseProcess";
  applicationName: Maybe<Scalars["String"]["output"]>;
  durationSeconds: Maybe<Scalars["Int"]["output"]>;
  pid: Maybe<Scalars["Int"]["output"]>;
  query: Maybe<Scalars["String"]["output"]>;
  state: Maybe<Scalars["String"]["output"]>;
  userName: Maybe<Scalars["String"]["output"]>;
  waitEvent: Maybe<Scalars["String"]["output"]>;
  waitEventType: Maybe<Scalars["String"]["output"]>;
};

export type DatabaseQueryResult = {
  __typename: "DatabaseQueryResult";
  columns: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  rowCount: Maybe<Scalars["Int"]["output"]>;
  rows: Maybe<Array<Maybe<DatabaseQueryRow>>>;
  truncated: Maybe<Scalars["Boolean"]["output"]>;
};

export type DatabaseQueryRow = {
  __typename: "DatabaseQueryRow";
  values: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
};

export type DatabaseRecoveryInfo = {
  __typename: "DatabaseRecoveryInfo";
  backups: Maybe<Array<Maybe<DatabaseBackup>>>;
  earliestRecoveryTime: Maybe<Scalars["String"]["output"]>;
  enabled: Maybe<Scalars["Boolean"]["output"]>;
  latestRecoveryTime: Maybe<Scalars["String"]["output"]>;
};

export type DatabaseSizeInfo = {
  __typename: "DatabaseSizeInfo";
  name: Maybe<Scalars["String"]["output"]>;
  sizeBytes: Maybe<Scalars["Int"]["output"]>;
  sizePretty: Maybe<Scalars["String"]["output"]>;
};

export type DatabaseSizes = {
  __typename: "DatabaseSizes";
  database: Maybe<DatabaseSizeInfo>;
  tables: Maybe<Array<Maybe<TableSizeInfo>>>;
};

export type DatabaseTableScan = {
  __typename: "DatabaseTableScan";
  deadRows: Maybe<Scalars["Int"]["output"]>;
  indexScanRows: Maybe<Scalars["Int"]["output"]>;
  indexScans: Maybe<Scalars["Int"]["output"]>;
  liveRows: Maybe<Scalars["Int"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
  schema: Maybe<Scalars["String"]["output"]>;
  seqScanRows: Maybe<Scalars["Int"]["output"]>;
  seqScans: Maybe<Scalars["Int"]["output"]>;
};

export type DatabaseTopQuery = {
  __typename: "DatabaseTopQuery";
  calls: Maybe<Scalars["Int"]["output"]>;
  meanTimeMs: Maybe<Scalars["Float"]["output"]>;
  query: Maybe<Scalars["String"]["output"]>;
  rows: Maybe<Scalars["Int"]["output"]>;
  sharedHitBlks: Maybe<Scalars["Int"]["output"]>;
  sharedReadBlks: Maybe<Scalars["Int"]["output"]>;
  totalTimeMs: Maybe<Scalars["Float"]["output"]>;
};

export type DatabaseUser = {
  __typename: "DatabaseUser";
  name: Maybe<Scalars["String"]["output"]>;
};

export type DatabaseUserWithPassword = {
  __typename: "DatabaseUserWithPassword";
  name: Maybe<Scalars["String"]["output"]>;
  password: Maybe<Scalars["String"]["output"]>;
};

export type DatastoreMetricsQueryInput = {
  end?: InputMaybe<Scalars["String"]["input"]>;
  kind?: InputMaybe<Scalars["String"]["input"]>;
  name: Scalars["String"]["input"];
  resolution?: InputMaybe<Scalars["Int"]["input"]>;
  resource: Scalars["String"]["input"];
  start?: InputMaybe<Scalars["String"]["input"]>;
};

export type Deploy = {
  __typename: "Deploy";
  commitCreatedAt: Maybe<Scalars["String"]["output"]>;
  commitId: Maybe<Scalars["String"]["output"]>;
  commitMessage: Maybe<Scalars["String"]["output"]>;
  createdAt: Maybe<Scalars["String"]["output"]>;
  failureReason: Maybe<Scalars["String"]["output"]>;
  finishedAt: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  image: Maybe<Scalars["String"]["output"]>;
  preDeployStatus: Maybe<Scalars["String"]["output"]>;
  rollbackOf: Maybe<Scalars["String"]["output"]>;
  serviceId: Maybe<Scalars["String"]["output"]>;
  startedAt: Maybe<Scalars["String"]["output"]>;
  status: Maybe<Scalars["String"]["output"]>;
  trigger: Maybe<Scalars["String"]["output"]>;
  updatedAt: Maybe<Scalars["String"]["output"]>;
};

export type DeployHook = {
  __typename: "DeployHook";
  url: Maybe<Scalars["String"]["output"]>;
};

export type DeployTrigger = {
  __typename: "DeployTrigger";
  clearCache: Maybe<Scalars["Boolean"]["output"]>;
  deployedByRender: Maybe<Scalars["Boolean"]["output"]>;
  envUpdated: Maybe<Scalars["Boolean"]["output"]>;
  firstBuild: Maybe<Scalars["Boolean"]["output"]>;
  manual: Maybe<Scalars["Boolean"]["output"]>;
  rollback: Maybe<Scalars["Boolean"]["output"]>;
};

export type EnvGroup = {
  __typename: "EnvGroup";
  createdAt: Maybe<Scalars["String"]["output"]>;
  envVars: Maybe<Array<Maybe<EnvGroupVar>>>;
  environmentId: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
  ownerId: Maybe<Scalars["String"]["output"]>;
  revision: Maybe<Scalars["String"]["output"]>;
  secretFiles: Maybe<Array<Maybe<EnvGroupSecretFile>>>;
  serviceLinks: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  updatedAt: Maybe<Scalars["String"]["output"]>;
};

export type EnvGroupEnvironmentPatchResult = {
  __typename: "EnvGroupEnvironmentPatchResult";
  affectedServiceIds: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  envVarKeys: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  failedServiceIds: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  revision: Maybe<Scalars["String"]["output"]>;
  rolledOut: Maybe<Scalars["Boolean"]["output"]>;
  secretFileNames: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
};

export type EnvGroupSecretFile = {
  __typename: "EnvGroupSecretFile";
  content: Maybe<Scalars["String"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
};

export type EnvGroupSecretFileInput = {
  content: Scalars["String"]["input"];
  name: Scalars["String"]["input"];
};

export type EnvGroupSecretFilePatchInput = {
  content?: InputMaybe<Scalars["String"]["input"]>;
  delete?: InputMaybe<Scalars["Boolean"]["input"]>;
  fromName?: InputMaybe<Scalars["String"]["input"]>;
  name: Scalars["String"]["input"];
};

export type EnvGroupVar = {
  __typename: "EnvGroupVar";
  key: Maybe<Scalars["String"]["output"]>;
  value: Maybe<Scalars["String"]["output"]>;
};

export type EnvGroupVarInput = {
  generateValue?: InputMaybe<Scalars["Boolean"]["input"]>;
  key: Scalars["String"]["input"];
  value?: InputMaybe<Scalars["String"]["input"]>;
};

export type EnvGroupVarPatchInput = {
  delete?: InputMaybe<Scalars["Boolean"]["input"]>;
  fromKey?: InputMaybe<Scalars["String"]["input"]>;
  generateValue?: InputMaybe<Scalars["Boolean"]["input"]>;
  key: Scalars["String"]["input"];
  value?: InputMaybe<Scalars["String"]["input"]>;
};

export type EnvVar = {
  __typename: "EnvVar";
  id: Maybe<Scalars["String"]["output"]>;
  key: Maybe<Scalars["String"]["output"]>;
  revision: Maybe<Scalars["String"]["output"]>;
  value: Maybe<Scalars["String"]["output"]>;
};

export type EnvVarInput = {
  generateValue?: InputMaybe<Scalars["Boolean"]["input"]>;
  key: Scalars["String"]["input"];
  value?: InputMaybe<Scalars["String"]["input"]>;
};

export type EnvVarListValue = {
  __typename: "EnvVarListValue";
  id: Maybe<Scalars["String"]["output"]>;
  key: Maybe<Scalars["String"]["output"]>;
  value: Maybe<Scalars["String"]["output"]>;
};

export type EnvVarWithCursor = {
  __typename: "EnvVarWithCursor";
  cursor: Maybe<Scalars["String"]["output"]>;
  envVar: Maybe<EnvVarListValue>;
};

export type Environment = {
  __typename: "Environment";
  createdAt: Maybe<Scalars["String"]["output"]>;
  databaseIds: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  envGroupIds: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  id: Maybe<Scalars["String"]["output"]>;
  ipAllowList: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  ipAllowListEntries: Maybe<Array<Maybe<IpAllowListEntry>>>;
  keyValueIds: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  name: Maybe<Scalars["String"]["output"]>;
  networkIsolationEnabled: Maybe<Scalars["Boolean"]["output"]>;
  ownerId: Maybe<Scalars["String"]["output"]>;
  projectId: Maybe<Scalars["String"]["output"]>;
  protectedStatus: Maybe<Scalars["String"]["output"]>;
  serviceIds: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
};

export type EnvironmentEnvVarPatchInput = {
  delete?: InputMaybe<Scalars["Boolean"]["input"]>;
  fromKey?: InputMaybe<Scalars["String"]["input"]>;
  generateValue?: InputMaybe<Scalars["Boolean"]["input"]>;
  key: Scalars["String"]["input"];
  value?: InputMaybe<Scalars["String"]["input"]>;
};

export type EnvironmentPatchResult = {
  __typename: "EnvironmentPatchResult";
  envVarKeys: Array<Scalars["String"]["output"]>;
  rolledOut: Scalars["Boolean"]["output"];
  secretFileNames: Array<Scalars["String"]["output"]>;
};

export type EnvironmentSecretFilePatchInput = {
  content?: InputMaybe<Scalars["String"]["input"]>;
  delete?: InputMaybe<Scalars["Boolean"]["input"]>;
  fromName?: InputMaybe<Scalars["String"]["input"]>;
  name: Scalars["String"]["input"];
};

export type EstimatedCost = {
  __typename: "EstimatedCost";
  meters: Maybe<Array<Maybe<MeterEstimate>>>;
  resources: Maybe<Array<Maybe<ResourceEstimate>>>;
  totalUsd: Maybe<Scalars["String"]["output"]>;
};

export type GeneratedBlueprint = {
  __typename: "GeneratedBlueprint";
  filename: Maybe<Scalars["String"]["output"]>;
  manifest: Maybe<Scalars["String"]["output"]>;
};

export type GitConnection = {
  __typename: "GitConnection";
  accountLogin: Maybe<Scalars["String"]["output"]>;
  connected: Maybe<Scalars["Boolean"]["output"]>;
  createdAt: Maybe<Scalars["String"]["output"]>;
  installUrl: Maybe<Scalars["String"]["output"]>;
  installationId: Maybe<Scalars["Float"]["output"]>;
};

export type IpAllowListEntry = {
  __typename: "IPAllowListEntry";
  cidrBlock: Scalars["String"]["output"];
  description: Maybe<Scalars["String"]["output"]>;
};

export type IpAllowListEntryInput = {
  cidrBlock: Scalars["String"]["input"];
  description?: InputMaybe<Scalars["String"]["input"]>;
};

export type InstanceType = {
  __typename: "InstanceType";
  cpu: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  memory: Maybe<Scalars["String"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
};

export type Job = {
  __typename: "Job";
  createdAt: Maybe<Scalars["String"]["output"]>;
  finishedAt: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  planId: Maybe<Scalars["String"]["output"]>;
  serviceId: Maybe<Scalars["String"]["output"]>;
  startCommand: Maybe<Scalars["String"]["output"]>;
  startedAt: Maybe<Scalars["String"]["output"]>;
  status: Maybe<Scalars["String"]["output"]>;
};

export type KeyValue = {
  __typename: "KeyValue";
  createdAt: Maybe<Scalars["String"]["output"]>;
  dashboardUrl: Maybe<Scalars["String"]["output"]>;
  environmentId: Maybe<Scalars["String"]["output"]>;
  externalHost: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  ipAllowList: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  ipAllowListEntries: Maybe<Array<Maybe<IpAllowListEntry>>>;
  maxmemoryPolicy: Maybe<Scalars["String"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
  ownerId: Maybe<Scalars["String"]["output"]>;
  persistenceMode: Maybe<Scalars["String"]["output"]>;
  plan: Maybe<Scalars["String"]["output"]>;
  projectId: Maybe<Scalars["String"]["output"]>;
  public: Maybe<Scalars["Boolean"]["output"]>;
  region: Maybe<Scalars["String"]["output"]>;
  status: Maybe<Scalars["String"]["output"]>;
  suspended: Maybe<Scalars["String"]["output"]>;
  updatedAt: Maybe<Scalars["String"]["output"]>;
  version: Maybe<Scalars["String"]["output"]>;
};

export type KeyValueConnectionInfo = {
  __typename: "KeyValueConnectionInfo";
  cliCommand: Maybe<Scalars["String"]["output"]>;
  externalConnectionString: Maybe<Scalars["String"]["output"]>;
  internalConnectionString: Maybe<Scalars["String"]["output"]>;
};

export type KeyValueInstanceType = {
  __typename: "KeyValueInstanceType";
  cpu: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  memory: Maybe<Scalars["String"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
  storageGB: Maybe<Scalars["Int"]["output"]>;
};

export type KeyValueLogEntry = {
  __typename: "KeyValueLogEntry";
  instance: Maybe<Scalars["String"]["output"]>;
  message: Maybe<Scalars["String"]["output"]>;
  timestamp: Maybe<Scalars["String"]["output"]>;
  type: Maybe<Scalars["String"]["output"]>;
};

export type LogEntry = {
  __typename: "LogEntry";
  instance: Maybe<Scalars["String"]["output"]>;
  level: Maybe<Scalars["String"]["output"]>;
  message: Maybe<Scalars["String"]["output"]>;
  method: Maybe<Scalars["String"]["output"]>;
  statusCode: Maybe<Scalars["String"]["output"]>;
  timestamp: Maybe<Scalars["String"]["output"]>;
  type: Maybe<Scalars["String"]["output"]>;
};

export type MaintenanceMode = {
  __typename: "MaintenanceMode";
  enabled: Scalars["Boolean"]["output"];
  uri: Scalars["String"]["output"];
};

export type MaintenanceModeInput = {
  enabled: Scalars["Boolean"]["input"];
  uri?: InputMaybe<Scalars["String"]["input"]>;
};

export type MeterEstimate = {
  __typename: "MeterEstimate";
  costUsd: Maybe<Scalars["String"]["output"]>;
  kind: Maybe<Scalars["String"]["output"]>;
  resourceKind: Maybe<Scalars["String"]["output"]>;
  tier: Maybe<Scalars["String"]["output"]>;
};

export type MetricLabel = {
  __typename: "MetricLabel";
  field: Maybe<Scalars["String"]["output"]>;
  value: Maybe<Scalars["String"]["output"]>;
};

export type MetricSeries = {
  __typename: "MetricSeries";
  labels: Maybe<Array<Maybe<MetricLabel>>>;
  parameters: Maybe<Array<Maybe<MetricSeriesParameter>>>;
  unit: Maybe<Scalars["String"]["output"]>;
  values: Maybe<Array<Maybe<MetricValue>>>;
};

export type MetricSeriesParameter = {
  __typename: "MetricSeriesParameter";
  quantile: Maybe<Scalars["Float"]["output"]>;
};

export type MetricValue = {
  __typename: "MetricValue";
  time: Maybe<Scalars["String"]["output"]>;
  value: Maybe<Scalars["Float"]["output"]>;
};

export type MetricsFilterInput = {
  field: Scalars["String"]["input"];
  values: Array<Scalars["String"]["input"]>;
};

export type MetricsFilterValues = {
  __typename: "MetricsFilterValues";
  field: Maybe<Scalars["String"]["output"]>;
  values: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
};

export type MetricsFiltersQueryInput = {
  end?: InputMaybe<Scalars["String"]["input"]>;
  filters: Array<MetricsFilterInput>;
  outputFilters: Array<Scalars["String"]["input"]>;
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
  start?: InputMaybe<Scalars["String"]["input"]>;
  type?: InputMaybe<Scalars["String"]["input"]>;
};

export type MetricsFiltersResult = {
  __typename: "MetricsFiltersResult";
  values: Maybe<Array<Maybe<MetricsFilterValues>>>;
};

export type MetricsParameterInput = {
  quantile?: InputMaybe<Scalars["Float"]["input"]>;
};

export type MetricsPathFilterSuggestions = {
  __typename: "MetricsPathFilterSuggestions";
  paths: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
};

export type MetricsPathFilterSuggestionsInput = {
  paths?: InputMaybe<Array<InputMaybe<Scalars["String"]["input"]>>>;
  serviceIDs: Array<Scalars["String"]["input"]>;
};

export type MetricsQueryInput = {
  aggregateAllMethod?: InputMaybe<Scalars["String"]["input"]>;
  aggregateBy?: InputMaybe<Array<InputMaybe<Scalars["String"]["input"]>>>;
  aggregationMethod?: InputMaybe<Scalars["String"]["input"]>;
  end?: InputMaybe<Scalars["String"]["input"]>;
  filters: Array<MetricsFilterInput>;
  name: Scalars["String"]["input"];
  parameters?: InputMaybe<Array<InputMaybe<MetricsParameterInput>>>;
  resolution?: InputMaybe<Scalars["Int"]["input"]>;
  start?: InputMaybe<Scalars["String"]["input"]>;
};

export type MonthToDateBandwidth = {
  __typename: "MonthToDateBandwidth";
  degradedSources: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  egressBandwidthMB: Maybe<Scalars["Float"]["output"]>;
  httpEgressBandwidthMB: Maybe<Scalars["Float"]["output"]>;
  natEgressBandwidthMB: Maybe<Scalars["Float"]["output"]>;
  privateLinkEgressBandwidthMB: Maybe<Scalars["Float"]["output"]>;
  websocketEgressBandwidthMB: Maybe<Scalars["Float"]["output"]>;
};

export type Mutation = {
  __typename: "Mutation";
  acceptWorkspaceInvite: Maybe<AcceptedWorkspaceInvite>;
  addCustomDomain: Maybe<CustomDomain>;
  archiveAgentSession: Maybe<AgentSession>;
  attachAgentSession: Maybe<AgentSession>;
  cancelAgentSession: Maybe<AgentSession>;
  cancelCronJobRun: Maybe<CronRun>;
  cancelDeploy: Maybe<Deploy>;
  cancelJob: Maybe<Job>;
  changeWorkspaceMemberRole: Maybe<WorkspaceMember>;
  changeWorkspacePlan: Maybe<Workspace>;
  cloneEnvGroup: Maybe<EnvGroup>;
  connectGit: Maybe<GitConnection>;
  createAgentSession: Maybe<AgentSession>;
  createApiKey: Maybe<ApiKey>;
  createBillingCheckoutSession: Maybe<BillingHostedSession>;
  createBillingPortalSession: Maybe<BillingHostedSession>;
  createBlueprint: Maybe<Blueprint>;
  createDatabase: Maybe<Database>;
  createDatabaseExport: Maybe<DatabaseExport>;
  createDatabaseUser: Maybe<DatabaseUserWithPassword>;
  createEnvGroup: Maybe<EnvGroup>;
  createEnvironment: Maybe<Environment>;
  createJob: Maybe<Job>;
  createKeyValue: Maybe<KeyValue>;
  createProject: Maybe<Project>;
  createRegistryCredential: Maybe<RegistryCredential>;
  createSSHKey: Maybe<SshKey>;
  createService: Maybe<Service>;
  createShellSession: Maybe<ShellSession>;
  createWebhookEndpoint: Maybe<WebhookEndpoint>;
  createWorkspace: Maybe<Workspace>;
  deleteAgentSession: Maybe<Scalars["Boolean"]["output"]>;
  deleteCustomDomain: Maybe<Scalars["Boolean"]["output"]>;
  deleteDatabase: Maybe<Scalars["Boolean"]["output"]>;
  deleteDatabaseUser: Maybe<Scalars["Boolean"]["output"]>;
  deleteEnvGroup: Maybe<Scalars["Boolean"]["output"]>;
  deleteEnvGroupSecretFile: Maybe<Scalars["Boolean"]["output"]>;
  deleteEnvGroupVar: Maybe<Scalars["Boolean"]["output"]>;
  deleteEnvVar: Maybe<Scalars["Boolean"]["output"]>;
  deleteEnvironment: Maybe<Scalars["String"]["output"]>;
  deleteKeyValue: Maybe<Scalars["Boolean"]["output"]>;
  deleteProject: Maybe<Scalars["String"]["output"]>;
  deleteRegistryCredential: Maybe<Scalars["Boolean"]["output"]>;
  deleteSSHKey: Scalars["Boolean"]["output"];
  deleteSecretFile: Maybe<Scalars["Boolean"]["output"]>;
  deleteService: Maybe<Scalars["Boolean"]["output"]>;
  deleteWebhookEndpoint: Maybe<Scalars["Boolean"]["output"]>;
  deleteWorkspace: Maybe<Scalars["String"]["output"]>;
  disableAutoscaling: Maybe<Scalars["Boolean"]["output"]>;
  disconnectBlueprint: Maybe<Scalars["Boolean"]["output"]>;
  disconnectGit: Maybe<Scalars["Boolean"]["output"]>;
  executeDatabaseQuery: Maybe<DatabaseQueryResult>;
  failoverDatabase: Maybe<Scalars["Boolean"]["output"]>;
  inviteWorkspaceMember: Maybe<WorkspaceInvite>;
  linkEnvGroup: Maybe<Scalars["Boolean"]["output"]>;
  markPushNotificationRead: Scalars["Boolean"]["output"];
  moveEnvGroup: Maybe<EnvGroup>;
  patchEnvGroupEnvironment: Maybe<EnvGroupEnvironmentPatchResult>;
  patchServiceEnvironment: EnvironmentPatchResult;
  pinAgentSession: Maybe<AgentSession>;
  recoverDatabase: Maybe<Database>;
  regenerateDeployHook: Maybe<DeployHook>;
  registerNotificationDeviceSubscription: Maybe<NotificationDeviceSubscription>;
  registerNotificationWebPushSubscription: Maybe<NotificationWebPushSubscription>;
  removeWorkspaceMember: Maybe<Scalars["String"]["output"]>;
  renameDatabase: Maybe<Database>;
  renameEnvGroup: Maybe<EnvGroup>;
  renameEnvironment: Maybe<Environment>;
  renameKeyValue: Maybe<KeyValue>;
  renameProject: Maybe<Project>;
  renameWorkspace: Maybe<Workspace>;
  resendWebhookDelivery: Maybe<WebhookDelivery>;
  resendWorkspaceInvite: Maybe<WorkspaceInvite>;
  restartDatabase: Maybe<Database>;
  restartServer: Maybe<Deploy>;
  resumeAgentSession: Maybe<AgentSession>;
  resumeDatabase: Maybe<Database>;
  resumeKeyValue: Maybe<KeyValue>;
  resumeService: Maybe<Service>;
  revokeApiKey: Maybe<Scalars["Boolean"]["output"]>;
  revokeNotificationDeviceSubscriptions: Maybe<Scalars["Int"]["output"]>;
  revokeWorkspaceInvite: Maybe<Scalars["String"]["output"]>;
  rollbackService: Maybe<Deploy>;
  runCronJob: Maybe<CronRun>;
  scaleService: Maybe<Service>;
  setAutoDeploy: Maybe<Service>;
  setAutoscaling: Maybe<Autoscaling>;
  setBranch: Maybe<Service>;
  setBuildCommand: Maybe<Service>;
  setBuildFilter: Maybe<Service>;
  setDatabaseIpAllowList: Maybe<Database>;
  setDatabaseParameterOverrides: Maybe<Database>;
  setDisplayName: Maybe<Service>;
  setDockerfilePath: Maybe<Service>;
  setEnvGroupSecretFile: Maybe<Scalars["Boolean"]["output"]>;
  setEnvGroupVar: Maybe<Scalars["Boolean"]["output"]>;
  setEnvGroupVars: Maybe<Scalars["Boolean"]["output"]>;
  setEnvVar: Maybe<Scalars["Boolean"]["output"]>;
  setEnvVars: Maybe<Scalars["Boolean"]["output"]>;
  setEnvironmentACL: Maybe<Environment>;
  setEnvironmentDatabases: Maybe<Environment>;
  setEnvironmentEnvGroups: Maybe<Environment>;
  setEnvironmentKeyValues: Maybe<Environment>;
  setEnvironmentServices: Maybe<Environment>;
  setHealthCheckPath: Maybe<Service>;
  setIdleTimeout: Maybe<Service>;
  setKeyValueIpAllowList: Maybe<KeyValue>;
  setKeyValueMaxmemoryPolicy: Maybe<KeyValue>;
  setMaintenanceMode: Maybe<Service>;
  setMaxShutdownDelay: Maybe<Service>;
  setNotificationsToSend: Maybe<Service>;
  setNotifyOnFail: Maybe<Service>;
  setPreDeployCommand: Maybe<Service>;
  setProjectDatabases: Maybe<Project>;
  setProjectKeyValues: Maybe<Project>;
  setProjectServices: Maybe<Project>;
  setPublishPath: Maybe<Service>;
  setRegistryCredential: Maybe<Service>;
  setRepo: Maybe<Service>;
  setRootDir: Maybe<Service>;
  setSecretFile: Maybe<Scalars["Boolean"]["output"]>;
  setServiceIpAllowList: Maybe<Service>;
  setStartCommand: Maybe<Service>;
  setStaticHeaders: Maybe<Service>;
  setStaticRoutes: Maybe<Service>;
  setSubdomainPolicy: Maybe<Service>;
  setWebhookEndpointEnabled: Maybe<WebhookEndpoint>;
  steerAgentSession: Maybe<AgentSession>;
  suspendDatabase: Maybe<Database>;
  suspendKeyValue: Maybe<KeyValue>;
  suspendService: Maybe<Service>;
  syncBlueprint: Maybe<SyncBlueprintResult>;
  triggerDeploy: Maybe<Deploy>;
  unarchiveAgentSession: Maybe<AgentSession>;
  unlinkEnvGroup: Maybe<Scalars["Boolean"]["output"]>;
  unpinAgentSession: Maybe<AgentSession>;
  unregisterNotificationDeviceSubscription: Maybe<Scalars["Boolean"]["output"]>;
  unregisterNotificationWebPushSubscription: Maybe<
    Scalars["Boolean"]["output"]
  >;
  updateBlueprint: Maybe<Blueprint>;
  updateCronJob: Maybe<Service>;
  updateDatabaseDiskAutoscaling: Maybe<Database>;
  updateDatabasePlan: Maybe<Database>;
  updateDatabaseVersion: Maybe<Database>;
  updateEnvironment: Maybe<Environment>;
  updateKeyValuePlan: Maybe<KeyValue>;
  updateNotificationSettings: Maybe<NotificationSettings>;
  updatePushNotificationSettings: Maybe<PushNotificationSettings>;
  updateRegistryCredential: Maybe<RegistryCredential>;
  updateServicePlan: Maybe<Service>;
  updateWebhookEndpoint: Maybe<WebhookEndpoint>;
  verifyCustomDomain: Maybe<CustomDomain>;
};

export type MutationAcceptWorkspaceInviteArgs = {
  token: Scalars["String"]["input"];
};

export type MutationAddCustomDomainArgs = {
  id: Scalars["String"]["input"];
  name: Scalars["String"]["input"];
};

export type MutationArchiveAgentSessionArgs = {
  id: Scalars["String"]["input"];
};

export type MutationAttachAgentSessionArgs = {
  action?: InputMaybe<Scalars["String"]["input"]>;
  id: Scalars["String"]["input"];
};

export type MutationCancelAgentSessionArgs = {
  id: Scalars["String"]["input"];
};

export type MutationCancelCronJobRunArgs = {
  runId: Scalars["String"]["input"];
  serviceId: Scalars["String"]["input"];
};

export type MutationCancelDeployArgs = {
  deployId: Scalars["String"]["input"];
  serviceId: Scalars["String"]["input"];
};

export type MutationCancelJobArgs = {
  jobId: Scalars["String"]["input"];
  serviceId: Scalars["String"]["input"];
};

export type MutationChangeWorkspaceMemberRoleArgs = {
  role: Scalars["String"]["input"];
  subject: Scalars["String"]["input"];
  workspaceId: Scalars["String"]["input"];
};

export type MutationChangeWorkspacePlanArgs = {
  id: Scalars["String"]["input"];
  plan: Scalars["String"]["input"];
};

export type MutationCloneEnvGroupArgs = {
  environmentId?: InputMaybe<Scalars["String"]["input"]>;
  id: Scalars["String"]["input"];
  name: Scalars["String"]["input"];
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationConnectGitArgs = {
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationCreateAgentSessionArgs = {
  agentConfig: AgentSessionConfigInput;
  branch: Scalars["String"]["input"];
  egressAllowlist?: InputMaybe<Array<Scalars["String"]["input"]>>;
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
  repo: Scalars["String"]["input"];
};

export type MutationCreateApiKeyArgs = {
  name: Scalars["String"]["input"];
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationCreateBillingCheckoutSessionArgs = {
  cancelUrl: Scalars["String"]["input"];
  successUrl: Scalars["String"]["input"];
  workspaceId: Scalars["String"]["input"];
};

export type MutationCreateBillingPortalSessionArgs = {
  returnUrl: Scalars["String"]["input"];
  workspaceId: Scalars["String"]["input"];
};

export type MutationCreateBlueprintArgs = {
  branch: Scalars["String"]["input"];
  confirm?: InputMaybe<Scalars["String"]["input"]>;
  envVarValues?: InputMaybe<Array<InputMaybe<BlueprintEnvVarValueInput>>>;
  name?: InputMaybe<Scalars["String"]["input"]>;
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
  path?: InputMaybe<Scalars["String"]["input"]>;
  repo: Scalars["String"]["input"];
};

export type MutationCreateDatabaseArgs = {
  databaseName?: InputMaybe<Scalars["String"]["input"]>;
  databaseUser?: InputMaybe<Scalars["String"]["input"]>;
  diskSizeGB?: InputMaybe<Scalars["Int"]["input"]>;
  dryRun?: InputMaybe<Scalars["Boolean"]["input"]>;
  enableDiskAutoscaling?: InputMaybe<Scalars["Boolean"]["input"]>;
  enableHighAvailability?: InputMaybe<Scalars["Boolean"]["input"]>;
  environmentId?: InputMaybe<Scalars["String"]["input"]>;
  ipAllowList?: InputMaybe<Array<InputMaybe<Scalars["String"]["input"]>>>;
  ipAllowListEntries?: InputMaybe<Array<IpAllowListEntryInput>>;
  name: Scalars["String"]["input"];
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
  plan?: InputMaybe<Scalars["String"]["input"]>;
  public?: InputMaybe<Scalars["Boolean"]["input"]>;
  version?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationCreateDatabaseExportArgs = {
  id: Scalars["String"]["input"];
};

export type MutationCreateDatabaseUserArgs = {
  id: Scalars["String"]["input"];
  name: Scalars["String"]["input"];
};

export type MutationCreateEnvGroupArgs = {
  envVars?: InputMaybe<Array<EnvGroupVarInput>>;
  environmentId?: InputMaybe<Scalars["String"]["input"]>;
  name: Scalars["String"]["input"];
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
  secretFiles?: InputMaybe<Array<EnvGroupSecretFileInput>>;
  serviceIds?: InputMaybe<Array<Scalars["String"]["input"]>>;
};

export type MutationCreateEnvironmentArgs = {
  ipAllowList?: InputMaybe<Array<IpAllowListEntryInput>>;
  name: Scalars["String"]["input"];
  networkIsolationEnabled?: InputMaybe<Scalars["Boolean"]["input"]>;
  projectId: Scalars["String"]["input"];
  protectedStatus?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationCreateJobArgs = {
  planId?: InputMaybe<Scalars["String"]["input"]>;
  serviceId: Scalars["String"]["input"];
  startCommand: Scalars["String"]["input"];
};

export type MutationCreateKeyValueArgs = {
  dryRun?: InputMaybe<Scalars["Boolean"]["input"]>;
  environmentId?: InputMaybe<Scalars["String"]["input"]>;
  ipAllowList?: InputMaybe<Array<InputMaybe<Scalars["String"]["input"]>>>;
  ipAllowListEntries?: InputMaybe<Array<IpAllowListEntryInput>>;
  maxmemoryPolicy?: InputMaybe<Scalars["String"]["input"]>;
  name: Scalars["String"]["input"];
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
  persistenceMode?: InputMaybe<Scalars["String"]["input"]>;
  plan?: InputMaybe<Scalars["String"]["input"]>;
  public?: InputMaybe<Scalars["Boolean"]["input"]>;
  storageGB?: InputMaybe<Scalars["Int"]["input"]>;
  version?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationCreateProjectArgs = {
  name: Scalars["String"]["input"];
  ownerId: Scalars["String"]["input"];
};

export type MutationCreateRegistryCredentialArgs = {
  authToken: Scalars["String"]["input"];
  expiresAt?: InputMaybe<Scalars["String"]["input"]>;
  host: Scalars["String"]["input"];
  name?: InputMaybe<Scalars["String"]["input"]>;
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
  username: Scalars["String"]["input"];
};

export type MutationCreateSshKeyArgs = {
  name: Scalars["String"]["input"];
  publicKey: Scalars["String"]["input"];
};

export type MutationCreateServiceArgs = {
  autoDeploy?: InputMaybe<Scalars["Boolean"]["input"]>;
  branch?: InputMaybe<Scalars["String"]["input"]>;
  buildCommand?: InputMaybe<Scalars["String"]["input"]>;
  buildFilter?: InputMaybe<BuildFilterInput>;
  builder?: InputMaybe<Scalars["String"]["input"]>;
  command?: InputMaybe<Scalars["String"]["input"]>;
  dockerfilePath?: InputMaybe<Scalars["String"]["input"]>;
  dryRun?: InputMaybe<Scalars["Boolean"]["input"]>;
  envVars?: InputMaybe<Array<InputMaybe<EnvVarInput>>>;
  environmentId?: InputMaybe<Scalars["String"]["input"]>;
  headers?: InputMaybe<Array<InputMaybe<StaticHeaderInput>>>;
  healthCheckPath?: InputMaybe<Scalars["String"]["input"]>;
  image?: InputMaybe<Scalars["String"]["input"]>;
  ipAllowList?: InputMaybe<Array<InputMaybe<Scalars["String"]["input"]>>>;
  ipAllowListEntries?: InputMaybe<Array<IpAllowListEntryInput>>;
  maintenanceMode?: InputMaybe<MaintenanceModeInput>;
  maxShutdownDelaySeconds?: InputMaybe<Scalars["Int"]["input"]>;
  name: Scalars["String"]["input"];
  notifyOnFail?: InputMaybe<Scalars["String"]["input"]>;
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
  plan?: InputMaybe<Scalars["String"]["input"]>;
  port?: InputMaybe<Scalars["Int"]["input"]>;
  preDeployCommand?: InputMaybe<Scalars["String"]["input"]>;
  publishPath?: InputMaybe<Scalars["String"]["input"]>;
  registryCredentialId?: InputMaybe<Scalars["String"]["input"]>;
  replicas?: InputMaybe<Scalars["Int"]["input"]>;
  repo?: InputMaybe<Scalars["String"]["input"]>;
  rootDir?: InputMaybe<Scalars["String"]["input"]>;
  routes?: InputMaybe<Array<InputMaybe<StaticRouteInput>>>;
  runtime?: InputMaybe<Scalars["String"]["input"]>;
  schedule?: InputMaybe<Scalars["String"]["input"]>;
  secretFiles?: InputMaybe<Array<InputMaybe<SecretFileInput>>>;
  startCommand?: InputMaybe<Scalars["String"]["input"]>;
  type?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationCreateShellSessionArgs = {
  id: Scalars["String"]["input"];
  instanceId?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationCreateWebhookEndpointArgs = {
  enabled?: InputMaybe<Scalars["Boolean"]["input"]>;
  eventTypes: Array<InputMaybe<Scalars["String"]["input"]>>;
  name?: InputMaybe<Scalars["String"]["input"]>;
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
  url: Scalars["String"]["input"];
};

export type MutationCreateWorkspaceArgs = {
  name: Scalars["String"]["input"];
  plan?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationDeleteAgentSessionArgs = {
  id: Scalars["String"]["input"];
};

export type MutationDeleteCustomDomainArgs = {
  id: Scalars["String"]["input"];
  name: Scalars["String"]["input"];
};

export type MutationDeleteDatabaseArgs = {
  confirm?: InputMaybe<Scalars["String"]["input"]>;
  id: Scalars["String"]["input"];
};

export type MutationDeleteDatabaseUserArgs = {
  id: Scalars["String"]["input"];
  name: Scalars["String"]["input"];
};

export type MutationDeleteEnvGroupArgs = {
  id: Scalars["String"]["input"];
};

export type MutationDeleteEnvGroupSecretFileArgs = {
  id: Scalars["String"]["input"];
  name: Scalars["String"]["input"];
};

export type MutationDeleteEnvGroupVarArgs = {
  id: Scalars["String"]["input"];
  key: Scalars["String"]["input"];
};

export type MutationDeleteEnvVarArgs = {
  key: Scalars["String"]["input"];
  serviceId: Scalars["String"]["input"];
};

export type MutationDeleteEnvironmentArgs = {
  id: Scalars["String"]["input"];
};

export type MutationDeleteKeyValueArgs = {
  confirm?: InputMaybe<Scalars["String"]["input"]>;
  id: Scalars["String"]["input"];
};

export type MutationDeleteProjectArgs = {
  id: Scalars["String"]["input"];
};

export type MutationDeleteRegistryCredentialArgs = {
  id: Scalars["String"]["input"];
};

export type MutationDeleteSshKeyArgs = {
  id: Scalars["String"]["input"];
};

export type MutationDeleteSecretFileArgs = {
  name: Scalars["String"]["input"];
  serviceId: Scalars["String"]["input"];
};

export type MutationDeleteServiceArgs = {
  confirm?: InputMaybe<Scalars["String"]["input"]>;
  id: Scalars["String"]["input"];
};

export type MutationDeleteWebhookEndpointArgs = {
  id: Scalars["String"]["input"];
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationDeleteWorkspaceArgs = {
  confirmation: Scalars["String"]["input"];
  id: Scalars["String"]["input"];
};

export type MutationDisableAutoscalingArgs = {
  id: Scalars["String"]["input"];
};

export type MutationDisconnectBlueprintArgs = {
  id: Scalars["String"]["input"];
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationDisconnectGitArgs = {
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationExecuteDatabaseQueryArgs = {
  allowWrites?: InputMaybe<Scalars["Boolean"]["input"]>;
  id: Scalars["String"]["input"];
  sql: Scalars["String"]["input"];
};

export type MutationFailoverDatabaseArgs = {
  id: Scalars["String"]["input"];
};

export type MutationInviteWorkspaceMemberArgs = {
  email: Scalars["String"]["input"];
  role: Scalars["String"]["input"];
  workspaceId: Scalars["String"]["input"];
};

export type MutationLinkEnvGroupArgs = {
  id: Scalars["String"]["input"];
  serviceId: Scalars["String"]["input"];
};

export type MutationMarkPushNotificationReadArgs = {
  id: Scalars["String"]["input"];
};

export type MutationMoveEnvGroupArgs = {
  environmentId: Scalars["String"]["input"];
  id: Scalars["String"]["input"];
};

export type MutationPatchEnvGroupEnvironmentArgs = {
  envVars?: InputMaybe<Array<EnvGroupVarPatchInput>>;
  expectedRevision?: InputMaybe<Scalars["String"]["input"]>;
  id: Scalars["String"]["input"];
  saveMode: Scalars["String"]["input"];
  secretFiles?: InputMaybe<Array<EnvGroupSecretFilePatchInput>>;
};

export type MutationPatchServiceEnvironmentArgs = {
  envVars?: InputMaybe<Array<EnvironmentEnvVarPatchInput>>;
  expectedEnvRevision?: InputMaybe<Scalars["String"]["input"]>;
  saveMode: Scalars["String"]["input"];
  secretFiles?: InputMaybe<Array<EnvironmentSecretFilePatchInput>>;
  serviceId: Scalars["String"]["input"];
};

export type MutationPinAgentSessionArgs = {
  id: Scalars["String"]["input"];
};

export type MutationRecoverDatabaseArgs = {
  id: Scalars["String"]["input"];
  name: Scalars["String"]["input"];
  plan?: InputMaybe<Scalars["String"]["input"]>;
  targetTime?: InputMaybe<Scalars["String"]["input"]>;
  version?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationRegenerateDeployHookArgs = {
  serviceId: Scalars["String"]["input"];
};

export type MutationRegisterNotificationDeviceSubscriptionArgs = {
  deviceId: Scalars["String"]["input"];
  platform: Scalars["String"]["input"];
  provider: Scalars["String"]["input"];
  sessionId: Scalars["String"]["input"];
  token: Scalars["String"]["input"];
};

export type MutationRegisterNotificationWebPushSubscriptionArgs = {
  auth: Scalars["String"]["input"];
  browserId: Scalars["String"]["input"];
  endpoint: Scalars["String"]["input"];
  p256dh: Scalars["String"]["input"];
};

export type MutationRemoveWorkspaceMemberArgs = {
  subject: Scalars["String"]["input"];
  workspaceId: Scalars["String"]["input"];
};

export type MutationRenameDatabaseArgs = {
  dryRun?: InputMaybe<Scalars["Boolean"]["input"]>;
  id: Scalars["String"]["input"];
  name: Scalars["String"]["input"];
};

export type MutationRenameEnvGroupArgs = {
  id: Scalars["String"]["input"];
  name: Scalars["String"]["input"];
};

export type MutationRenameEnvironmentArgs = {
  id: Scalars["String"]["input"];
  name: Scalars["String"]["input"];
};

export type MutationRenameKeyValueArgs = {
  dryRun?: InputMaybe<Scalars["Boolean"]["input"]>;
  id: Scalars["String"]["input"];
  name: Scalars["String"]["input"];
};

export type MutationRenameProjectArgs = {
  id: Scalars["String"]["input"];
  name: Scalars["String"]["input"];
};

export type MutationRenameWorkspaceArgs = {
  id: Scalars["String"]["input"];
  name: Scalars["String"]["input"];
};

export type MutationResendWebhookDeliveryArgs = {
  attemptId: Scalars["String"]["input"];
  endpointId: Scalars["String"]["input"];
  idempotencyKey: Scalars["String"]["input"];
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationResendWorkspaceInviteArgs = {
  inviteId: Scalars["String"]["input"];
  workspaceId: Scalars["String"]["input"];
};

export type MutationRestartDatabaseArgs = {
  id: Scalars["String"]["input"];
};

export type MutationRestartServerArgs = {
  serviceId: Scalars["String"]["input"];
};

export type MutationResumeAgentSessionArgs = {
  id: Scalars["String"]["input"];
};

export type MutationResumeDatabaseArgs = {
  id: Scalars["String"]["input"];
};

export type MutationResumeKeyValueArgs = {
  id: Scalars["String"]["input"];
};

export type MutationResumeServiceArgs = {
  id: Scalars["String"]["input"];
};

export type MutationRevokeApiKeyArgs = {
  id: Scalars["String"]["input"];
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationRevokeWorkspaceInviteArgs = {
  inviteId: Scalars["String"]["input"];
  workspaceId: Scalars["String"]["input"];
};

export type MutationRollbackServiceArgs = {
  deployId: Scalars["String"]["input"];
  serviceId: Scalars["String"]["input"];
};

export type MutationRunCronJobArgs = {
  id: Scalars["String"]["input"];
};

export type MutationScaleServiceArgs = {
  id: Scalars["String"]["input"];
  numInstances: Scalars["Int"]["input"];
};

export type MutationSetAutoDeployArgs = {
  enabled: Scalars["Boolean"]["input"];
  id: Scalars["String"]["input"];
};

export type MutationSetAutoscalingArgs = {
  id: Scalars["String"]["input"];
  maxInstances: Scalars["Int"]["input"];
  minInstances: Scalars["Int"]["input"];
  targetCPUPercent?: InputMaybe<Scalars["Int"]["input"]>;
  targetMemoryPercent?: InputMaybe<Scalars["Int"]["input"]>;
};

export type MutationSetBranchArgs = {
  branch: Scalars["String"]["input"];
  id: Scalars["String"]["input"];
};

export type MutationSetBuildCommandArgs = {
  command: Scalars["String"]["input"];
  id: Scalars["String"]["input"];
};

export type MutationSetBuildFilterArgs = {
  buildFilter: BuildFilterInput;
  id: Scalars["String"]["input"];
};

export type MutationSetDatabaseIpAllowListArgs = {
  cidrs?: InputMaybe<Array<InputMaybe<Scalars["String"]["input"]>>>;
  entries?: InputMaybe<Array<IpAllowListEntryInput>>;
  id: Scalars["String"]["input"];
};

export type MutationSetDatabaseParameterOverridesArgs = {
  id: Scalars["String"]["input"];
  parameters?: InputMaybe<Array<InputMaybe<ParameterInput>>>;
};

export type MutationSetDisplayNameArgs = {
  displayName: Scalars["String"]["input"];
  id: Scalars["String"]["input"];
};

export type MutationSetDockerfilePathArgs = {
  dockerfilePath: Scalars["String"]["input"];
  id: Scalars["String"]["input"];
};

export type MutationSetEnvGroupSecretFileArgs = {
  content?: InputMaybe<Scalars["String"]["input"]>;
  id: Scalars["String"]["input"];
  name: Scalars["String"]["input"];
};

export type MutationSetEnvGroupVarArgs = {
  generateValue?: InputMaybe<Scalars["Boolean"]["input"]>;
  id: Scalars["String"]["input"];
  key: Scalars["String"]["input"];
  value?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationSetEnvGroupVarsArgs = {
  envVars: Array<EnvGroupVarInput>;
  id: Scalars["String"]["input"];
};

export type MutationSetEnvVarArgs = {
  generateValue?: InputMaybe<Scalars["Boolean"]["input"]>;
  key: Scalars["String"]["input"];
  serviceId: Scalars["String"]["input"];
  value?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationSetEnvVarsArgs = {
  envVars: Array<EnvVarInput>;
  serviceId: Scalars["String"]["input"];
};

export type MutationSetEnvironmentAclArgs = {
  id: Scalars["String"]["input"];
  ipAllowList?: InputMaybe<Array<Scalars["String"]["input"]>>;
  ipAllowListEntries?: InputMaybe<Array<IpAllowListEntryInput>>;
  networkIsolationEnabled: Scalars["Boolean"]["input"];
  protectedStatus: Scalars["String"]["input"];
};

export type MutationSetEnvironmentDatabasesArgs = {
  databaseIds: Array<Scalars["String"]["input"]>;
  id: Scalars["String"]["input"];
};

export type MutationSetEnvironmentEnvGroupsArgs = {
  envGroupIds: Array<Scalars["String"]["input"]>;
  id: Scalars["String"]["input"];
};

export type MutationSetEnvironmentKeyValuesArgs = {
  id: Scalars["String"]["input"];
  keyValueIds: Array<Scalars["String"]["input"]>;
};

export type MutationSetEnvironmentServicesArgs = {
  id: Scalars["String"]["input"];
  serviceIds: Array<Scalars["String"]["input"]>;
};

export type MutationSetHealthCheckPathArgs = {
  id: Scalars["String"]["input"];
  path: Scalars["String"]["input"];
};

export type MutationSetIdleTimeoutArgs = {
  id: Scalars["String"]["input"];
  idleTTLSeconds: Scalars["Int"]["input"];
};

export type MutationSetKeyValueIpAllowListArgs = {
  cidrs?: InputMaybe<Array<InputMaybe<Scalars["String"]["input"]>>>;
  entries?: InputMaybe<Array<IpAllowListEntryInput>>;
  id: Scalars["String"]["input"];
};

export type MutationSetKeyValueMaxmemoryPolicyArgs = {
  dryRun?: InputMaybe<Scalars["Boolean"]["input"]>;
  id: Scalars["String"]["input"];
  maxmemoryPolicy: Scalars["String"]["input"];
};

export type MutationSetMaintenanceModeArgs = {
  id: Scalars["String"]["input"];
  maintenanceMode: MaintenanceModeInput;
};

export type MutationSetMaxShutdownDelayArgs = {
  id: Scalars["String"]["input"];
  seconds: Scalars["Int"]["input"];
};

export type MutationSetNotificationsToSendArgs = {
  id: Scalars["String"]["input"];
  value: Scalars["String"]["input"];
};

export type MutationSetNotifyOnFailArgs = {
  id: Scalars["String"]["input"];
  value: Scalars["String"]["input"];
};

export type MutationSetPreDeployCommandArgs = {
  command: Scalars["String"]["input"];
  id: Scalars["String"]["input"];
};

export type MutationSetProjectDatabasesArgs = {
  databaseIds: Array<Scalars["String"]["input"]>;
  id: Scalars["String"]["input"];
};

export type MutationSetProjectKeyValuesArgs = {
  id: Scalars["String"]["input"];
  keyValueIds: Array<Scalars["String"]["input"]>;
};

export type MutationSetProjectServicesArgs = {
  id: Scalars["String"]["input"];
  serviceIds: Array<Scalars["String"]["input"]>;
};

export type MutationSetPublishPathArgs = {
  id: Scalars["String"]["input"];
  publishPath: Scalars["String"]["input"];
};

export type MutationSetRegistryCredentialArgs = {
  id: Scalars["String"]["input"];
  registryCredentialId: Scalars["String"]["input"];
};

export type MutationSetRepoArgs = {
  id: Scalars["String"]["input"];
  repo: Scalars["String"]["input"];
};

export type MutationSetRootDirArgs = {
  id: Scalars["String"]["input"];
  rootDir: Scalars["String"]["input"];
};

export type MutationSetSecretFileArgs = {
  content?: InputMaybe<Scalars["String"]["input"]>;
  name: Scalars["String"]["input"];
  serviceId: Scalars["String"]["input"];
};

export type MutationSetServiceIpAllowListArgs = {
  cidrs?: InputMaybe<Array<InputMaybe<Scalars["String"]["input"]>>>;
  entries?: InputMaybe<Array<IpAllowListEntryInput>>;
  id: Scalars["String"]["input"];
};

export type MutationSetStartCommandArgs = {
  command: Scalars["String"]["input"];
  id: Scalars["String"]["input"];
};

export type MutationSetStaticHeadersArgs = {
  headers?: InputMaybe<Array<InputMaybe<StaticHeaderInput>>>;
  id: Scalars["String"]["input"];
};

export type MutationSetStaticRoutesArgs = {
  id: Scalars["String"]["input"];
  routes?: InputMaybe<Array<InputMaybe<StaticRouteInput>>>;
};

export type MutationSetSubdomainPolicyArgs = {
  id: Scalars["String"]["input"];
  policy: Scalars["String"]["input"];
};

export type MutationSetWebhookEndpointEnabledArgs = {
  enabled: Scalars["Boolean"]["input"];
  id: Scalars["String"]["input"];
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationSteerAgentSessionArgs = {
  egressAllowlist?: InputMaybe<Array<Scalars["String"]["input"]>>;
  id: Scalars["String"]["input"];
  prompt: Scalars["String"]["input"];
};

export type MutationSuspendDatabaseArgs = {
  confirm?: InputMaybe<Scalars["String"]["input"]>;
  id: Scalars["String"]["input"];
};

export type MutationSuspendKeyValueArgs = {
  confirm?: InputMaybe<Scalars["String"]["input"]>;
  id: Scalars["String"]["input"];
};

export type MutationSuspendServiceArgs = {
  confirm?: InputMaybe<Scalars["String"]["input"]>;
  id: Scalars["String"]["input"];
};

export type MutationSyncBlueprintArgs = {
  bexYaml?: InputMaybe<Scalars["String"]["input"]>;
  confirm?: InputMaybe<Scalars["String"]["input"]>;
  id: Scalars["String"]["input"];
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationTriggerDeployArgs = {
  commitId?: InputMaybe<Scalars["String"]["input"]>;
  deployMode?: InputMaybe<Scalars["String"]["input"]>;
  imageUrl?: InputMaybe<Scalars["String"]["input"]>;
  serviceId: Scalars["String"]["input"];
};

export type MutationUnarchiveAgentSessionArgs = {
  id: Scalars["String"]["input"];
};

export type MutationUnlinkEnvGroupArgs = {
  id: Scalars["String"]["input"];
  serviceId: Scalars["String"]["input"];
};

export type MutationUnpinAgentSessionArgs = {
  id: Scalars["String"]["input"];
};

export type MutationUnregisterNotificationDeviceSubscriptionArgs = {
  deviceId: Scalars["String"]["input"];
};

export type MutationUnregisterNotificationWebPushSubscriptionArgs = {
  browserId: Scalars["String"]["input"];
};

export type MutationUpdateBlueprintArgs = {
  autoSync?: InputMaybe<Scalars["Boolean"]["input"]>;
  id: Scalars["String"]["input"];
  name?: InputMaybe<Scalars["String"]["input"]>;
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
  path?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationUpdateCronJobArgs = {
  command?: InputMaybe<Scalars["String"]["input"]>;
  id: Scalars["String"]["input"];
  schedule: Scalars["String"]["input"];
};

export type MutationUpdateDatabaseDiskAutoscalingArgs = {
  enabled: Scalars["Boolean"]["input"];
  id: Scalars["String"]["input"];
};

export type MutationUpdateDatabasePlanArgs = {
  dryRun?: InputMaybe<Scalars["Boolean"]["input"]>;
  id: Scalars["String"]["input"];
  plan: Scalars["String"]["input"];
};

export type MutationUpdateDatabaseVersionArgs = {
  id: Scalars["String"]["input"];
  version: Scalars["String"]["input"];
};

export type MutationUpdateEnvironmentArgs = {
  id: Scalars["String"]["input"];
  ipAllowList?: InputMaybe<Array<Scalars["String"]["input"]>>;
  ipAllowListEntries?: InputMaybe<Array<IpAllowListEntryInput>>;
  name?: InputMaybe<Scalars["String"]["input"]>;
  networkIsolationEnabled?: InputMaybe<Scalars["Boolean"]["input"]>;
  protectedStatus?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationUpdateKeyValuePlanArgs = {
  dryRun?: InputMaybe<Scalars["Boolean"]["input"]>;
  id: Scalars["String"]["input"];
  plan: Scalars["String"]["input"];
};

export type MutationUpdateNotificationSettingsArgs = {
  deployFailed: Scalars["Boolean"]["input"];
  deployStarted: Scalars["Boolean"]["input"];
  deploySucceeded: Scalars["Boolean"]["input"];
};

export type MutationUpdatePushNotificationSettingsArgs = {
  settings: PushNotificationSettingsInput;
};

export type MutationUpdateRegistryCredentialArgs = {
  authToken?: InputMaybe<Scalars["String"]["input"]>;
  expiresAt?: InputMaybe<Scalars["String"]["input"]>;
  id: Scalars["String"]["input"];
  name?: InputMaybe<Scalars["String"]["input"]>;
  username?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationUpdateServicePlanArgs = {
  dryRun?: InputMaybe<Scalars["Boolean"]["input"]>;
  id: Scalars["String"]["input"];
  plan: Scalars["String"]["input"];
};

export type MutationUpdateWebhookEndpointArgs = {
  enabled?: InputMaybe<Scalars["Boolean"]["input"]>;
  eventTypes?: InputMaybe<Array<InputMaybe<Scalars["String"]["input"]>>>;
  id: Scalars["String"]["input"];
  name?: InputMaybe<Scalars["String"]["input"]>;
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
  url?: InputMaybe<Scalars["String"]["input"]>;
};

export type MutationVerifyCustomDomainArgs = {
  id: Scalars["String"]["input"];
  name: Scalars["String"]["input"];
};

export type NameAvailability = {
  __typename: "NameAvailability";
  available: Maybe<Scalars["Boolean"]["output"]>;
  suggestion: Maybe<Scalars["String"]["output"]>;
};

export type NotificationDeviceSubscription = {
  __typename: "NotificationDeviceSubscription";
  createdAt: Maybe<Scalars["String"]["output"]>;
  deviceId: Maybe<Scalars["String"]["output"]>;
  lastRegisteredAt: Maybe<Scalars["String"]["output"]>;
  platform: Maybe<Scalars["String"]["output"]>;
  preferenceRef: Maybe<Scalars["String"]["output"]>;
  provider: Maybe<Scalars["String"]["output"]>;
  updatedAt: Maybe<Scalars["String"]["output"]>;
};

export type NotificationWebPushSubscription = {
  __typename: "NotificationWebPushSubscription";
  browserId: Maybe<Scalars["String"]["output"]>;
  createdAt: Maybe<Scalars["String"]["output"]>;
  lastRegisteredAt: Maybe<Scalars["String"]["output"]>;
  updatedAt: Maybe<Scalars["String"]["output"]>;
};

export type NotificationSettings = {
  __typename: "NotificationSettings";
  deployFailed: Maybe<Scalars["Boolean"]["output"]>;
  deployStarted: Maybe<Scalars["Boolean"]["output"]>;
  deploySucceeded: Maybe<Scalars["Boolean"]["output"]>;
};

export type ParameterInput = {
  name: Scalars["String"]["input"];
  value: Scalars["String"]["input"];
};

export type PostgresConnectionInfo = {
  __typename: "PostgresConnectionInfo";
  externalConnectionPoolString: Maybe<Scalars["String"]["output"]>;
  externalConnectionString: Maybe<Scalars["String"]["output"]>;
  internalConnectionPoolString: Maybe<Scalars["String"]["output"]>;
  internalConnectionString: Maybe<Scalars["String"]["output"]>;
  password: Maybe<Scalars["String"]["output"]>;
  psqlCommand: Maybe<Scalars["String"]["output"]>;
  readReplicaConnectionStrings: Maybe<Array<Maybe<ReplicaConnectionStrings>>>;
};

export type Project = {
  __typename: "Project";
  createdAt: Maybe<Scalars["String"]["output"]>;
  databaseIds: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  id: Maybe<Scalars["String"]["output"]>;
  keyValueIds: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  name: Maybe<Scalars["String"]["output"]>;
  ownerId: Maybe<Scalars["String"]["output"]>;
  serviceIds: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
};

export type PushNotification = {
  __typename: "PushNotification";
  body: Scalars["String"]["output"];
  createdAt: Scalars["String"]["output"];
  deepLink: Scalars["String"]["output"];
  event: PushNotificationEvent;
  id: Scalars["String"]["output"];
  occurredAt: Scalars["String"]["output"];
  readAt: Maybe<Scalars["String"]["output"]>;
  resourceId: Scalars["String"]["output"];
  resourceKind: Scalars["String"]["output"];
  title: Scalars["String"]["output"];
  urgency: PushNotificationUrgency;
};

export type PushNotificationClockRange = {
  __typename: "PushNotificationClockRange";
  end: Scalars["String"]["output"];
  start: Scalars["String"]["output"];
  weekdays: Array<PushNotificationWeekday>;
};

export type PushNotificationClockRangeInput = {
  end: Scalars["String"]["input"];
  start: Scalars["String"]["input"];
  weekdays: Array<PushNotificationWeekday>;
};

export enum PushNotificationEvent {
  AgentFailed = "AGENT_FAILED",
  AgentNeedsDecision = "AGENT_NEEDS_DECISION",
  AgentPrReady = "AGENT_PR_READY",
  CronFailed = "CRON_FAILED",
  DeployFailed = "DEPLOY_FAILED",
  DeployStarted = "DEPLOY_STARTED",
  DeploySucceeded = "DEPLOY_SUCCEEDED",
  ServerAvailable = "SERVER_AVAILABLE",
  ServerFailed = "SERVER_FAILED",
  ServiceResumed = "SERVICE_RESUMED",
  ServiceSuspended = "SERVICE_SUSPENDED",
}

export type PushNotificationServiceOverride = {
  __typename: "PushNotificationServiceOverride";
  enabled: Maybe<Scalars["Boolean"]["output"]>;
  events: Maybe<Array<PushNotificationEvent>>;
  minimumUrgency: Maybe<PushNotificationUrgency>;
  serviceId: Scalars["String"]["output"];
};

export type PushNotificationServiceOverrideInput = {
  enabled?: InputMaybe<Scalars["Boolean"]["input"]>;
  events?: InputMaybe<Array<PushNotificationEvent>>;
  minimumUrgency?: InputMaybe<PushNotificationUrgency>;
  serviceId: Scalars["String"]["input"];
};

export type PushNotificationSettings = {
  __typename: "PushNotificationSettings";
  enabled: Scalars["Boolean"]["output"];
  events: Array<PushNotificationEvent>;
  maxDeferralSeconds: Scalars["Int"]["output"];
  minimumUrgency: PushNotificationUrgency;
  quietHours: Array<PushNotificationClockRange>;
  serviceOverrides: Array<PushNotificationServiceOverride>;
  timeZone: Scalars["String"]["output"];
  workingHours: Array<PushNotificationClockRange>;
};

export type PushNotificationSettingsInput = {
  enabled: Scalars["Boolean"]["input"];
  events: Array<PushNotificationEvent>;
  maxDeferralSeconds: Scalars["Int"]["input"];
  minimumUrgency: PushNotificationUrgency;
  quietHours: Array<PushNotificationClockRangeInput>;
  serviceOverrides: Array<PushNotificationServiceOverrideInput>;
  timeZone: Scalars["String"]["input"];
  workingHours: Array<PushNotificationClockRangeInput>;
};

export enum PushNotificationUrgency {
  Critical = "CRITICAL",
  Important = "IMPORTANT",
  Routine = "ROUTINE",
}

export enum PushNotificationWeekday {
  Friday = "FRIDAY",
  Monday = "MONDAY",
  Saturday = "SATURDAY",
  Sunday = "SUNDAY",
  Thursday = "THURSDAY",
  Tuesday = "TUESDAY",
  Wednesday = "WEDNESDAY",
}

export type Query = {
  __typename: "Query";
  agentSession: Maybe<AgentSession>;
  agentSessionCapabilities: AgentSessionCapabilities;
  agentSessionTranscript: AgentSessionTranscriptPage;
  agentSessions: Array<AgentSession>;
  apiKeys: Maybe<Array<Maybe<ApiKey>>>;
  auditLogs: Maybe<Array<Maybe<AuditLog>>>;
  autoscalingConfig: Maybe<Autoscaling>;
  blueprint: Maybe<Blueprint>;
  blueprintPreview: Maybe<BlueprintPreview>;
  blueprintSyncs: Maybe<Array<Maybe<BlueprintSync>>>;
  blueprints: Maybe<Array<Maybe<Blueprint>>>;
  cronJobRun: Maybe<CronRun>;
  cronJobRuns: Maybe<Array<Maybe<CronRun>>>;
  customDomain: Maybe<CustomDomain>;
  customDomains: Maybe<Array<Maybe<CustomDomain>>>;
  database: Maybe<Database>;
  databaseConnectionInfo: Maybe<PostgresConnectionInfo>;
  databaseExports: Maybe<Array<Maybe<DatabaseExport>>>;
  databaseInstanceTypes: Maybe<Array<Maybe<DatabaseInstanceType>>>;
  databaseIpAllowList: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  databaseLogs: Maybe<Array<Maybe<DatabaseLogEntry>>>;
  databaseParameterOverrides: Maybe<Array<Maybe<DatabaseParameterOverride>>>;
  databaseProcesses: Maybe<Array<Maybe<DatabaseProcess>>>;
  databaseRecoveryInfo: Maybe<DatabaseRecoveryInfo>;
  databaseSizes: Maybe<DatabaseSizes>;
  databaseTableScans: Maybe<Array<Maybe<DatabaseTableScan>>>;
  databaseTopQueries: Maybe<Array<Maybe<DatabaseTopQuery>>>;
  databaseUsers: Maybe<Array<Maybe<DatabaseUser>>>;
  databases: Maybe<Array<Maybe<Database>>>;
  datastoreMetrics: Maybe<Array<Maybe<MetricSeries>>>;
  deploy: Maybe<Deploy>;
  deployHook: Maybe<DeployHook>;
  deploys: Maybe<Array<Maybe<Deploy>>>;
  envGroup: Maybe<EnvGroup>;
  envGroupSecretFile: Maybe<EnvGroupSecretFile>;
  envGroupVar: Maybe<EnvGroupVar>;
  envGroups: Maybe<Array<Maybe<EnvGroup>>>;
  envVars: Maybe<Array<Maybe<EnvVarWithCursor>>>;
  environment: Maybe<Environment>;
  environments: Maybe<Array<Maybe<Environment>>>;
  generateBlueprint: Maybe<GeneratedBlueprint>;
  gitConnection: Maybe<GitConnection>;
  instanceTypes: Maybe<Array<Maybe<InstanceType>>>;
  job: Maybe<Job>;
  jobs: Maybe<Array<Maybe<Job>>>;
  keyValue: Maybe<KeyValue>;
  keyValueConnectionInfo: Maybe<KeyValueConnectionInfo>;
  keyValueInstanceTypes: Maybe<Array<Maybe<KeyValueInstanceType>>>;
  keyValueIpAllowList: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  keyValueLogs: Maybe<Array<Maybe<KeyValueLogEntry>>>;
  keyValues: Maybe<Array<Maybe<KeyValue>>>;
  logLabelValues: Maybe<Array<Scalars["String"]["output"]>>;
  logs: Maybe<Array<Maybe<LogEntry>>>;
  metrics: Maybe<Array<Maybe<MetricSeries>>>;
  metricsFilters: Maybe<MetricsFiltersResult>;
  metricsPathFilterSuggestions: Maybe<MetricsPathFilterSuggestions>;
  monthToDateBandwidth: Maybe<MonthToDateBandwidth>;
  notificationDeviceSubscriptions: Maybe<
    Array<Maybe<NotificationDeviceSubscription>>
  >;
  notificationInbox: Array<PushNotification>;
  notificationSettings: Maybe<NotificationSettings>;
  notificationWebPushSubscriptions: Maybe<
    Array<Maybe<NotificationWebPushSubscription>>
  >;
  project: Maybe<Project>;
  projects: Maybe<Array<Maybe<Project>>>;
  pushNotificationSettings: Maybe<PushNotificationSettings>;
  pushNotificationsAvailable: Scalars["Boolean"]["output"];
  webPushAvailable: Scalars["Boolean"]["output"];
  webPushVapidPublicKey: Maybe<Scalars["String"]["output"]>;
  registryCredential: Maybe<RegistryCredential>;
  registryCredentials: Maybe<Array<Maybe<RegistryCredential>>>;
  repoBranches: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  repos: Maybe<Array<Maybe<Repo>>>;
  secretFiles: Maybe<Array<Maybe<SecretFileWithCursor>>>;
  server: Maybe<Service>;
  service: Maybe<Service>;
  serviceEvent: Maybe<ServiceEvent>;
  serviceEvents: Maybe<Array<Maybe<ServiceEvent>>>;
  serviceInstances: Maybe<Array<Maybe<ServiceInstance>>>;
  serviceNameAvailable: Maybe<NameAvailability>;
  services: Maybe<Array<Maybe<Service>>>;
  sshKeys: Maybe<Array<Maybe<SshKey>>>;
  unreadPushNotificationCount: Scalars["Int"]["output"];
  usage: Maybe<UsageSummary>;
  validateBlueprint: Maybe<BlueprintValidation>;
  viewerCapabilities: Maybe<ViewerCapabilities>;
  webhookDeliveries: Maybe<Array<Maybe<WebhookDelivery>>>;
  webhookEndpoint: Maybe<WebhookEndpoint>;
  webhookEndpoints: Maybe<Array<Maybe<WebhookEndpoint>>>;
  webhookEventTypes: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  workspaceBillingReadiness: Maybe<WorkspaceBillingReadiness>;
  workspaceEnvironments: Maybe<Array<Maybe<Environment>>>;
  workspaceInvites: Maybe<Array<Maybe<WorkspaceInvite>>>;
  workspaceLimits: Maybe<ResourceLimits>;
  workspaceMembers: Maybe<Array<Maybe<WorkspaceMember>>>;
  workspaceSeatUsage: Maybe<WorkspaceSeatUsage>;
  workspaces: Maybe<Array<Maybe<Workspace>>>;
};

export type QueryAgentSessionArgs = {
  id: Scalars["String"]["input"];
};

export type QueryAgentSessionCapabilitiesArgs = {
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryAgentSessionTranscriptArgs = {
  afterSeq?: InputMaybe<Scalars["Int"]["input"]>;
  id: Scalars["String"]["input"];
  limit?: InputMaybe<Scalars["Int"]["input"]>;
};

export type QueryAgentSessionsArgs = {
  archived?: InputMaybe<Scalars["String"]["input"]>;
  createdAfter?: InputMaybe<Scalars["String"]["input"]>;
  createdBefore?: InputMaybe<Scalars["String"]["input"]>;
  cursor?: InputMaybe<Scalars["String"]["input"]>;
  limit?: InputMaybe<Scalars["Int"]["input"]>;
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
  phases?: InputMaybe<Array<Scalars["String"]["input"]>>;
  repo?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryApiKeysArgs = {
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryAuditLogsArgs = {
  cursor?: InputMaybe<Scalars["String"]["input"]>;
  direction?: InputMaybe<Scalars["String"]["input"]>;
  endTime?: InputMaybe<Scalars["String"]["input"]>;
  limit?: InputMaybe<Scalars["Int"]["input"]>;
  ownerId: Scalars["String"]["input"];
  startTime?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryAutoscalingConfigArgs = {
  id: Scalars["String"]["input"];
};

export type QueryBlueprintArgs = {
  id: Scalars["String"]["input"];
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryBlueprintPreviewArgs = {
  branch: Scalars["String"]["input"];
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
  path?: InputMaybe<Scalars["String"]["input"]>;
  repo: Scalars["String"]["input"];
};

export type QueryBlueprintSyncsArgs = {
  cursor?: InputMaybe<Scalars["String"]["input"]>;
  id: Scalars["String"]["input"];
  limit?: InputMaybe<Scalars["Int"]["input"]>;
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryBlueprintsArgs = {
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryCronJobRunArgs = {
  runId: Scalars["String"]["input"];
  serviceId: Scalars["String"]["input"];
};

export type QueryCronJobRunsArgs = {
  cursor?: InputMaybe<Scalars["String"]["input"]>;
  limit?: InputMaybe<Scalars["Int"]["input"]>;
  serviceId: Scalars["String"]["input"];
};

export type QueryCustomDomainArgs = {
  id: Scalars["String"]["input"];
  name: Scalars["String"]["input"];
};

export type QueryCustomDomainsArgs = {
  cursor?: InputMaybe<Scalars["String"]["input"]>;
  domainType?: InputMaybe<Scalars["String"]["input"]>;
  id: Scalars["String"]["input"];
  limit?: InputMaybe<Scalars["Int"]["input"]>;
  verificationStatus?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryDatabaseArgs = {
  id: Scalars["String"]["input"];
};

export type QueryDatabaseConnectionInfoArgs = {
  id: Scalars["String"]["input"];
};

export type QueryDatabaseExportsArgs = {
  id: Scalars["String"]["input"];
};

export type QueryDatabaseIpAllowListArgs = {
  id: Scalars["String"]["input"];
};

export type QueryDatabaseLogsArgs = {
  cursor?: InputMaybe<Scalars["String"]["input"]>;
  direction?: InputMaybe<Scalars["String"]["input"]>;
  endTime?: InputMaybe<Scalars["String"]["input"]>;
  id: Scalars["String"]["input"];
  instance?: InputMaybe<Array<InputMaybe<Scalars["String"]["input"]>>>;
  limit?: InputMaybe<Scalars["Int"]["input"]>;
  startTime?: InputMaybe<Scalars["String"]["input"]>;
  text?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryDatabaseParameterOverridesArgs = {
  id: Scalars["String"]["input"];
};

export type QueryDatabaseProcessesArgs = {
  id: Scalars["String"]["input"];
};

export type QueryDatabaseRecoveryInfoArgs = {
  id: Scalars["String"]["input"];
};

export type QueryDatabaseSizesArgs = {
  id: Scalars["String"]["input"];
};

export type QueryDatabaseTableScansArgs = {
  id: Scalars["String"]["input"];
};

export type QueryDatabaseTopQueriesArgs = {
  id: Scalars["String"]["input"];
};

export type QueryDatabaseUsersArgs = {
  id: Scalars["String"]["input"];
};

export type QueryDatabasesArgs = {
  cursor?: InputMaybe<Scalars["String"]["input"]>;
  limit?: InputMaybe<Scalars["Int"]["input"]>;
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryDatastoreMetricsArgs = {
  query: DatastoreMetricsQueryInput;
};

export type QueryDeployArgs = {
  deployId: Scalars["String"]["input"];
  serviceId: Scalars["String"]["input"];
};

export type QueryDeployHookArgs = {
  serviceId: Scalars["String"]["input"];
};

export type QueryDeploysArgs = {
  createdAfter?: InputMaybe<Scalars["String"]["input"]>;
  createdBefore?: InputMaybe<Scalars["String"]["input"]>;
  cursor?: InputMaybe<Scalars["String"]["input"]>;
  finishedAfter?: InputMaybe<Scalars["String"]["input"]>;
  finishedBefore?: InputMaybe<Scalars["String"]["input"]>;
  limit?: InputMaybe<Scalars["Int"]["input"]>;
  serviceId: Scalars["String"]["input"];
  status?: InputMaybe<Array<InputMaybe<Scalars["String"]["input"]>>>;
  updatedAfter?: InputMaybe<Scalars["String"]["input"]>;
  updatedBefore?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryEnvGroupArgs = {
  id: Scalars["String"]["input"];
};

export type QueryEnvGroupSecretFileArgs = {
  id: Scalars["String"]["input"];
  name: Scalars["String"]["input"];
};

export type QueryEnvGroupVarArgs = {
  id: Scalars["String"]["input"];
  key: Scalars["String"]["input"];
};

export type QueryEnvGroupsArgs = {
  cursor?: InputMaybe<Scalars["String"]["input"]>;
  limit?: InputMaybe<Scalars["Int"]["input"]>;
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryEnvVarsArgs = {
  cursor?: InputMaybe<Scalars["String"]["input"]>;
  limit?: InputMaybe<Scalars["Int"]["input"]>;
  serviceId: Scalars["String"]["input"];
};

export type QueryEnvironmentArgs = {
  id: Scalars["String"]["input"];
};

export type QueryEnvironmentsArgs = {
  cursor?: InputMaybe<Scalars["String"]["input"]>;
  limit?: InputMaybe<Scalars["Int"]["input"]>;
  projectId: Scalars["String"]["input"];
};

export type QueryGenerateBlueprintArgs = {
  keyValueIds?: InputMaybe<Array<InputMaybe<Scalars["String"]["input"]>>>;
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
  postgresIds?: InputMaybe<Array<InputMaybe<Scalars["String"]["input"]>>>;
  serviceIds?: InputMaybe<Array<InputMaybe<Scalars["String"]["input"]>>>;
};

export type QueryGitConnectionArgs = {
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryJobArgs = {
  jobId: Scalars["String"]["input"];
  serviceId: Scalars["String"]["input"];
};

export type QueryJobsArgs = {
  cursor?: InputMaybe<Scalars["String"]["input"]>;
  limit?: InputMaybe<Scalars["Int"]["input"]>;
  serviceId: Scalars["String"]["input"];
};

export type QueryKeyValueArgs = {
  id: Scalars["String"]["input"];
};

export type QueryKeyValueConnectionInfoArgs = {
  id: Scalars["String"]["input"];
};

export type QueryKeyValueIpAllowListArgs = {
  id: Scalars["String"]["input"];
};

export type QueryKeyValueLogsArgs = {
  cursor?: InputMaybe<Scalars["String"]["input"]>;
  direction?: InputMaybe<Scalars["String"]["input"]>;
  endTime?: InputMaybe<Scalars["String"]["input"]>;
  id: Scalars["String"]["input"];
  instance?: InputMaybe<Array<InputMaybe<Scalars["String"]["input"]>>>;
  limit?: InputMaybe<Scalars["Int"]["input"]>;
  startTime?: InputMaybe<Scalars["String"]["input"]>;
  text?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryKeyValuesArgs = {
  cursor?: InputMaybe<Scalars["String"]["input"]>;
  limit?: InputMaybe<Scalars["Int"]["input"]>;
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryLogLabelValuesArgs = {
  direction?: InputMaybe<Scalars["String"]["input"]>;
  endTime?: InputMaybe<Scalars["String"]["input"]>;
  host?: InputMaybe<Array<Scalars["String"]["input"]>>;
  instance?: InputMaybe<Array<Scalars["String"]["input"]>>;
  label: Scalars["String"]["input"];
  level?: InputMaybe<Array<Scalars["String"]["input"]>>;
  method?: InputMaybe<Array<Scalars["String"]["input"]>>;
  path?: InputMaybe<Array<Scalars["String"]["input"]>>;
  resource: Scalars["String"]["input"];
  startTime?: InputMaybe<Scalars["String"]["input"]>;
  statusCode?: InputMaybe<Array<Scalars["String"]["input"]>>;
  text?: InputMaybe<Scalars["String"]["input"]>;
  type?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryLogsArgs = {
  direction?: InputMaybe<Scalars["String"]["input"]>;
  endTime?: InputMaybe<Scalars["String"]["input"]>;
  host?: InputMaybe<Array<Scalars["String"]["input"]>>;
  instance?: InputMaybe<Array<Scalars["String"]["input"]>>;
  level?: InputMaybe<Array<Scalars["String"]["input"]>>;
  limit?: InputMaybe<Scalars["Int"]["input"]>;
  method?: InputMaybe<Array<Scalars["String"]["input"]>>;
  path?: InputMaybe<Array<Scalars["String"]["input"]>>;
  resource: Scalars["String"]["input"];
  startTime?: InputMaybe<Scalars["String"]["input"]>;
  statusCode?: InputMaybe<Array<Scalars["String"]["input"]>>;
  text?: InputMaybe<Scalars["String"]["input"]>;
  type?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryMetricsArgs = {
  query: MetricsQueryInput;
};

export type QueryMetricsFiltersArgs = {
  query: MetricsFiltersQueryInput;
};

export type QueryMetricsPathFilterSuggestionsArgs = {
  query: MetricsPathFilterSuggestionsInput;
};

export type QueryMonthToDateBandwidthArgs = {
  resourceId: Scalars["String"]["input"];
};

export type QueryNotificationInboxArgs = {
  limit?: InputMaybe<Scalars["Int"]["input"]>;
};

export type QueryProjectArgs = {
  id: Scalars["String"]["input"];
};

export type QueryProjectsArgs = {
  cursor?: InputMaybe<Scalars["String"]["input"]>;
  limit?: InputMaybe<Scalars["Int"]["input"]>;
  ownerId: Scalars["String"]["input"];
};

export type QueryRegistryCredentialArgs = {
  id: Scalars["String"]["input"];
};

export type QueryRegistryCredentialsArgs = {
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryRepoBranchesArgs = {
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
  repo: Scalars["String"]["input"];
};

export type QueryReposArgs = {
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type QuerySecretFilesArgs = {
  cursor?: InputMaybe<Scalars["String"]["input"]>;
  limit?: InputMaybe<Scalars["Int"]["input"]>;
  serviceId: Scalars["String"]["input"];
};

export type QueryServerArgs = {
  id: Scalars["String"]["input"];
};

export type QueryServiceArgs = {
  id: Scalars["String"]["input"];
};

export type QueryServiceEventArgs = {
  id: Scalars["String"]["input"];
};

export type QueryServiceEventsArgs = {
  cursor?: InputMaybe<Scalars["String"]["input"]>;
  endTime?: InputMaybe<Scalars["String"]["input"]>;
  limit?: InputMaybe<Scalars["Int"]["input"]>;
  serviceId: Scalars["String"]["input"];
  startTime?: InputMaybe<Scalars["String"]["input"]>;
  type?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryServiceInstancesArgs = {
  id: Scalars["String"]["input"];
};

export type QueryServiceNameAvailableArgs = {
  name: Scalars["String"]["input"];
};

export type QueryServicesArgs = {
  cursor?: InputMaybe<Scalars["String"]["input"]>;
  limit?: InputMaybe<Scalars["Int"]["input"]>;
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryUsageArgs = {
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
  period?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryValidateBlueprintArgs = {
  bexYaml: Scalars["String"]["input"];
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryViewerCapabilitiesArgs = {
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryWebhookDeliveriesArgs = {
  cursor?: InputMaybe<Scalars["String"]["input"]>;
  endpointId: Scalars["String"]["input"];
  limit?: InputMaybe<Scalars["Int"]["input"]>;
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
  sentAfter?: InputMaybe<Scalars["String"]["input"]>;
  sentBefore?: InputMaybe<Scalars["String"]["input"]>;
  status?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryWebhookEndpointArgs = {
  id: Scalars["String"]["input"];
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryWebhookEndpointsArgs = {
  ownerId?: InputMaybe<Scalars["String"]["input"]>;
};

export type QueryWorkspaceBillingReadinessArgs = {
  workspaceId: Scalars["String"]["input"];
};

export type QueryWorkspaceEnvironmentsArgs = {
  cursor?: InputMaybe<Scalars["String"]["input"]>;
  limit?: InputMaybe<Scalars["Int"]["input"]>;
  ownerId: Scalars["String"]["input"];
};

export type QueryWorkspaceInvitesArgs = {
  workspaceId: Scalars["String"]["input"];
};

export type QueryWorkspaceLimitsArgs = {
  ownerId: Scalars["String"]["input"];
};

export type QueryWorkspaceMembersArgs = {
  workspaceId: Scalars["String"]["input"];
};

export type QueryWorkspaceSeatUsageArgs = {
  workspaceId: Scalars["String"]["input"];
};

export type ReadReplicaConnectionInfo = {
  __typename: "ReadReplicaConnectionInfo";
  externalHost: Maybe<Scalars["String"]["output"]>;
  internalHost: Maybe<Scalars["String"]["output"]>;
};

export type ReadReplicaView = {
  __typename: "ReadReplicaView";
  connectionInfo: Maybe<ReadReplicaConnectionInfo>;
  name: Maybe<Scalars["String"]["output"]>;
};

export type RegistryCredential = {
  __typename: "RegistryCredential";
  createdAt: Maybe<Scalars["String"]["output"]>;
  expiresAt: Maybe<Scalars["String"]["output"]>;
  host: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
  ownerId: Maybe<Scalars["String"]["output"]>;
  status: Maybe<Scalars["String"]["output"]>;
  updatedAt: Maybe<Scalars["String"]["output"]>;
  username: Maybe<Scalars["String"]["output"]>;
};

export type ReplicaConnectionStrings = {
  __typename: "ReplicaConnectionStrings";
  externalConnectionString: Maybe<Scalars["String"]["output"]>;
  internalConnectionString: Maybe<Scalars["String"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
};

export type Repo = {
  __typename: "Repo";
  cloneUrl: Maybe<Scalars["String"]["output"]>;
  defaultBranch: Maybe<Scalars["String"]["output"]>;
  fullName: Maybe<Scalars["String"]["output"]>;
  htmlUrl: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["Float"]["output"]>;
  private: Maybe<Scalars["Boolean"]["output"]>;
};

export type ResourceCap = {
  __typename: "ResourceCap";
  limit: Maybe<Scalars["Int"]["output"]>;
  used: Maybe<Scalars["Int"]["output"]>;
};

export type ResourceEstimate = {
  __typename: "ResourceEstimate";
  charges: Maybe<Array<Maybe<ChargeLine>>>;
  costUsd: Maybe<Scalars["String"]["output"]>;
  resourceKind: Maybe<Scalars["String"]["output"]>;
  serviceId: Maybe<Scalars["String"]["output"]>;
  serviceName: Maybe<Scalars["String"]["output"]>;
};

export type ResourceLimits = {
  __typename: "ResourceLimits";
  keyValues: Maybe<ResourceCap>;
  postgres: Maybe<ResourceCap>;
  services: Maybe<ResourceCap>;
};

export type SshKey = {
  __typename: "SSHKey";
  createdAt: Scalars["String"]["output"];
  fingerprint: Scalars["String"]["output"];
  id: Scalars["String"]["output"];
  name: Scalars["String"]["output"];
  publicKey: Scalars["String"]["output"];
};

export type SecretFile = {
  __typename: "SecretFile";
  content: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
};

export type SecretFileInput = {
  content: Scalars["String"]["input"];
  name: Scalars["String"]["input"];
};

export type SecretFileListValue = {
  __typename: "SecretFileListValue";
  id: Maybe<Scalars["String"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
};

export type SecretFileWithCursor = {
  __typename: "SecretFileWithCursor";
  cursor: Maybe<Scalars["String"]["output"]>;
  secretFile: Maybe<SecretFileListValue>;
};

export type Service = {
  __typename: "Service";
  autoDeploy: Maybe<Scalars["Boolean"]["output"]>;
  autoDeployTrigger: Maybe<Scalars["String"]["output"]>;
  autoscaling: Maybe<Autoscaling>;
  branch: Maybe<Scalars["String"]["output"]>;
  buildCommand: Maybe<Scalars["String"]["output"]>;
  buildFilter: Maybe<BuildFilter>;
  builder: Maybe<Scalars["String"]["output"]>;
  command: Maybe<Scalars["String"]["output"]>;
  createdAt: Maybe<Scalars["String"]["output"]>;
  dashboardUrl: Maybe<Scalars["String"]["output"]>;
  displayName: Maybe<Scalars["String"]["output"]>;
  dockerfilePath: Maybe<Scalars["String"]["output"]>;
  envVar: Maybe<EnvVar>;
  envVarKeys: Maybe<Array<Maybe<EnvVar>>>;
  environmentId: Maybe<Scalars["String"]["output"]>;
  headers: Maybe<Array<Maybe<StaticHeader>>>;
  healthCheckPath: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  idleTTLSeconds: Maybe<Scalars["Int"]["output"]>;
  internalAddress: Maybe<Scalars["String"]["output"]>;
  ipAllowList: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  ipAllowListEntries: Maybe<Array<Maybe<IpAllowListEntry>>>;
  lastSuccessfulRunAt: Maybe<Scalars["String"]["output"]>;
  latestDeployId: Maybe<Scalars["String"]["output"]>;
  maintenanceMode: MaintenanceMode;
  maxShutdownDelaySeconds: Maybe<Scalars["Int"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
  notificationsToSend: Maybe<Scalars["String"]["output"]>;
  notifyOnFail: Maybe<Scalars["String"]["output"]>;
  ownerId: Maybe<Scalars["String"]["output"]>;
  phase: Maybe<Scalars["String"]["output"]>;
  plan: Maybe<Scalars["String"]["output"]>;
  preDeployCommand: Maybe<Scalars["String"]["output"]>;
  projectId: Maybe<Scalars["String"]["output"]>;
  publicRoutingNotice: Maybe<Scalars["String"]["output"]>;
  publishPath: Maybe<Scalars["String"]["output"]>;
  region: Maybe<Scalars["String"]["output"]>;
  registryCredentialId: Maybe<Scalars["String"]["output"]>;
  renderSubdomainPolicy: Maybe<Scalars["String"]["output"]>;
  replicas: Maybe<Scalars["Int"]["output"]>;
  repo: Maybe<Scalars["String"]["output"]>;
  revision: Maybe<Scalars["String"]["output"]>;
  rootDir: Maybe<Scalars["String"]["output"]>;
  routes: Maybe<Array<Maybe<StaticRoute>>>;
  runs: Maybe<Array<Maybe<CronRun>>>;
  runtime: Maybe<Scalars["String"]["output"]>;
  schedule: Maybe<Scalars["String"]["output"]>;
  secretFile: Maybe<SecretFile>;
  secretFileNames: Maybe<Array<Maybe<SecretFile>>>;
  slug: Maybe<Scalars["String"]["output"]>;
  sshAddress: Maybe<Scalars["String"]["output"]>;
  startCommand: Maybe<Scalars["String"]["output"]>;
  suspended: Maybe<Scalars["String"]["output"]>;
  suspenders: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  type: Maybe<Scalars["String"]["output"]>;
  updatedAt: Maybe<Scalars["String"]["output"]>;
  url: Maybe<Scalars["String"]["output"]>;
};

export type ServiceEnvVarArgs = {
  key: Scalars["String"]["input"];
};

export type ServiceSecretFileArgs = {
  name: Scalars["String"]["input"];
};

export type ServiceEvent = {
  __typename: "ServiceEvent";
  cursor: Maybe<Scalars["String"]["output"]>;
  details: Maybe<ServiceEventDetails>;
  id: Maybe<Scalars["String"]["output"]>;
  serviceId: Maybe<Scalars["String"]["output"]>;
  timestamp: Maybe<Scalars["String"]["output"]>;
  type: Maybe<Scalars["String"]["output"]>;
};

export type ServiceEventDetails = {
  __typename: "ServiceEventDetails";
  actor: Maybe<Scalars["String"]["output"]>;
  autoscalingMaxFrom: Maybe<Scalars["Int"]["output"]>;
  autoscalingMaxTo: Maybe<Scalars["Int"]["output"]>;
  autoscalingMinFrom: Maybe<Scalars["Int"]["output"]>;
  autoscalingMinTo: Maybe<Scalars["Int"]["output"]>;
  branchFrom: Maybe<Scalars["String"]["output"]>;
  branchTo: Maybe<Scalars["String"]["output"]>;
  commitId: Maybe<Scalars["String"]["output"]>;
  commitMessage: Maybe<Scalars["String"]["output"]>;
  commitUrl: Maybe<Scalars["String"]["output"]>;
  deployId: Maybe<Scalars["String"]["output"]>;
  deployStatus: Maybe<Scalars["String"]["output"]>;
  finishedAt: Maybe<Scalars["String"]["output"]>;
  fromCount: Maybe<Scalars["Int"]["output"]>;
  image: Maybe<Scalars["String"]["output"]>;
  instanceCountFrom: Maybe<Scalars["Int"]["output"]>;
  instanceCountTo: Maybe<Scalars["Int"]["output"]>;
  instanceId: Maybe<Scalars["String"]["output"]>;
  planFrom: Maybe<Scalars["String"]["output"]>;
  planTo: Maybe<Scalars["String"]["output"]>;
  preDeployStatus: Maybe<Scalars["String"]["output"]>;
  reasonCode: Maybe<Scalars["String"]["output"]>;
  startedAt: Maybe<Scalars["String"]["output"]>;
  status: Maybe<Scalars["String"]["output"]>;
  toCount: Maybe<Scalars["Int"]["output"]>;
  trigger: Maybe<DeployTrigger>;
  triggeredByUser: Maybe<Scalars["String"]["output"]>;
};

export type ServiceInstance = {
  __typename: "ServiceInstance";
  createdAt: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
};

export type ServiceUsage = {
  __typename: "ServiceUsage";
  resourceKind: Maybe<Scalars["String"]["output"]>;
  rows: Maybe<Array<Maybe<UsageRow>>>;
  serviceId: Maybe<Scalars["String"]["output"]>;
  serviceName: Maybe<Scalars["String"]["output"]>;
};

export type ShellSession = {
  __typename: "ShellSession";
  expiresAt: Maybe<Scalars["String"]["output"]>;
  ticket: Maybe<Scalars["String"]["output"]>;
  url: Maybe<Scalars["String"]["output"]>;
};

export type StaticHeader = {
  __typename: "StaticHeader";
  name: Maybe<Scalars["String"]["output"]>;
  path: Maybe<Scalars["String"]["output"]>;
  value: Maybe<Scalars["String"]["output"]>;
};

export type StaticHeaderInput = {
  name: Scalars["String"]["input"];
  path: Scalars["String"]["input"];
  value: Scalars["String"]["input"];
};

export type StaticRoute = {
  __typename: "StaticRoute";
  destination: Maybe<Scalars["String"]["output"]>;
  source: Maybe<Scalars["String"]["output"]>;
  type: Maybe<Scalars["String"]["output"]>;
};

export type StaticRouteInput = {
  destination: Scalars["String"]["input"];
  source: Scalars["String"]["input"];
  type: Scalars["String"]["input"];
};

export type SyncBlueprintResult = {
  __typename: "SyncBlueprintResult";
  blueprint: Maybe<Blueprint>;
  databases: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  services: Maybe<Array<Maybe<Service>>>;
};

export type TableSizeInfo = {
  __typename: "TableSizeInfo";
  name: Maybe<Scalars["String"]["output"]>;
  schema: Maybe<Scalars["String"]["output"]>;
  sizeBytes: Maybe<Scalars["Int"]["output"]>;
  sizePretty: Maybe<Scalars["String"]["output"]>;
};

export type UsageCoverage = {
  __typename: "UsageCoverage";
  degradedSources: Array<Scalars["String"]["output"]>;
  state: Scalars["String"]["output"];
  through: Maybe<Scalars["String"]["output"]>;
};

export type UsageRow = {
  __typename: "UsageRow";
  kind: Maybe<Scalars["String"]["output"]>;
  tier: Maybe<Scalars["String"]["output"]>;
  total: Maybe<Scalars["Float"]["output"]>;
};

export type UsageSummary = {
  __typename: "UsageSummary";
  billing: Maybe<Billing>;
  coverage: UsageCoverage;
  estimatedCost: Maybe<EstimatedCost>;
  period: Maybe<Scalars["String"]["output"]>;
  services: Maybe<Array<Maybe<ServiceUsage>>>;
  workspaceId: Maybe<Scalars["String"]["output"]>;
};

export type ViewerCapabilities = {
  __typename: "ViewerCapabilities";
  canCreate: Scalars["Boolean"]["output"];
  canManage: Scalars["Boolean"]["output"];
  canManageBilling: Scalars["Boolean"]["output"];
  canManageKeys: Scalars["Boolean"]["output"];
  canOperate: Scalars["Boolean"]["output"];
  canView: Scalars["Boolean"]["output"];
  canViewLogs: Scalars["Boolean"]["output"];
  canViewSensitive: Scalars["Boolean"]["output"];
  role: Maybe<Scalars["String"]["output"]>;
};

export type WebhookDelivery = {
  __typename: "WebhookDelivery";
  attemptNumber: Maybe<Scalars["Int"]["output"]>;
  cursor: Maybe<Scalars["String"]["output"]>;
  eventId: Maybe<Scalars["String"]["output"]>;
  eventType: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  nextAttemptAt: Maybe<Scalars["String"]["output"]>;
  parentStatus: Maybe<Scalars["String"]["output"]>;
  requestBody: Maybe<Scalars["String"]["output"]>;
  responseBody: Maybe<Scalars["String"]["output"]>;
  sentAt: Maybe<Scalars["String"]["output"]>;
  serviceId: Maybe<Scalars["String"]["output"]>;
  status: Maybe<Scalars["String"]["output"]>;
  statusCode: Maybe<Scalars["Int"]["output"]>;
  transportError: Maybe<Scalars["String"]["output"]>;
};

export type WebhookEndpoint = {
  __typename: "WebhookEndpoint";
  createdAt: Maybe<Scalars["String"]["output"]>;
  createdBy: Maybe<Scalars["String"]["output"]>;
  disabledReason: Maybe<Scalars["String"]["output"]>;
  enabled: Maybe<Scalars["Boolean"]["output"]>;
  eventTypes: Maybe<Array<Maybe<Scalars["String"]["output"]>>>;
  id: Maybe<Scalars["String"]["output"]>;
  latestParentStatus: Maybe<Scalars["String"]["output"]>;
  latestSentAt: Maybe<Scalars["String"]["output"]>;
  latestStatus: Maybe<Scalars["String"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
  ownerId: Maybe<Scalars["String"]["output"]>;
  secret: Maybe<Scalars["String"]["output"]>;
  updatedAt: Maybe<Scalars["String"]["output"]>;
  url: Maybe<Scalars["String"]["output"]>;
};

export type Workspace = {
  __typename: "Workspace";
  createdAt: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  name: Maybe<Scalars["String"]["output"]>;
  plan: Maybe<Scalars["String"]["output"]>;
  role: Maybe<Scalars["String"]["output"]>;
};

export type WorkspaceBillingReadiness = {
  __typename: "WorkspaceBillingReadiness";
  customerReady: Maybe<Scalars["Boolean"]["output"]>;
  lifecycle: Maybe<BillingLifecycle>;
  mode: Maybe<Scalars["String"]["output"]>;
  paymentMethodBrand: Maybe<Scalars["String"]["output"]>;
  paymentMethodLast4: Maybe<Scalars["String"]["output"]>;
  paymentMethodReady: Maybe<Scalars["Boolean"]["output"]>;
  paymentMethodRequired: Maybe<Scalars["Boolean"]["output"]>;
  subscriptionReady: Maybe<Scalars["Boolean"]["output"]>;
  tax: Maybe<BillingTaxReadiness>;
  workspaceId: Maybe<Scalars["String"]["output"]>;
};

export type WorkspaceInvite = {
  __typename: "WorkspaceInvite";
  createdAt: Maybe<Scalars["String"]["output"]>;
  email: Maybe<Scalars["String"]["output"]>;
  expiresAt: Maybe<Scalars["String"]["output"]>;
  id: Maybe<Scalars["String"]["output"]>;
  role: Maybe<Scalars["String"]["output"]>;
};

export type WorkspaceMember = {
  __typename: "WorkspaceMember";
  createdAt: Maybe<Scalars["String"]["output"]>;
  email: Maybe<Scalars["String"]["output"]>;
  mfaEnabled: Maybe<Scalars["Boolean"]["output"]>;
  role: Maybe<Scalars["String"]["output"]>;
  subject: Maybe<Scalars["String"]["output"]>;
  userId: Maybe<Scalars["String"]["output"]>;
};

export type WorkspaceSeatUsage = {
  __typename: "WorkspaceSeatUsage";
  limit: Maybe<Scalars["Int"]["output"]>;
  used: Maybe<Scalars["Int"]["output"]>;
};
