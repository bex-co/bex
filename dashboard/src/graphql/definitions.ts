import type { TypedDocumentNode as DocumentNode } from '@graphql-typed-document-node/core';
export type Maybe<T> = T | null;
export type InputMaybe<T> = Maybe<T>;
export type Exact<T extends { [key: string]: unknown }> = { [K in keyof T]: T[K] };
export type MakeOptional<T, K extends keyof T> = Omit<T, K> & { [SubKey in K]?: Maybe<T[SubKey]> };
export type MakeMaybe<T, K extends keyof T> = Omit<T, K> & { [SubKey in K]: Maybe<T[SubKey]> };
export type MakeEmpty<T extends { [key: string]: unknown }, K extends keyof T> = { [_ in K]?: never };
export type Incremental<T> = T | { [P in keyof T]?: P extends ' $fragmentName' | '__typename' ? T[P] : never };
/** All built-in and custom scalars, mapped to their actual values */
export type Scalars = {
  ID: { input: string; output: string; }
  String: { input: string; output: string; }
  Boolean: { input: boolean; output: boolean; }
  Int: { input: number; output: number; }
  Float: { input: number; output: number; }
};

export type AcceptedWorkspaceInvite = {
  __typename: 'AcceptedWorkspaceInvite';
  role: Maybe<Scalars['String']['output']>;
  workspaceId: Maybe<Scalars['String']['output']>;
  workspaceName: Maybe<Scalars['String']['output']>;
};

export type ApiKey = {
  __typename: 'ApiKey';
  createdAt: Maybe<Scalars['String']['output']>;
  createdBy: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  lastUsedAt: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
  secret: Maybe<Scalars['String']['output']>;
};

export type AuditLog = {
  __typename: 'AuditLog';
  action: Maybe<Scalars['String']['output']>;
  actor: Maybe<Scalars['String']['output']>;
  actorMethod: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  metadata: Maybe<AuditLogMetadata>;
  resource: Maybe<Scalars['String']['output']>;
  status: Maybe<Scalars['String']['output']>;
  targetName: Maybe<Scalars['String']['output']>;
  timestamp: Maybe<Scalars['String']['output']>;
};

export type AuditLogMetadata = {
  __typename: 'AuditLogMetadata';
  to: Maybe<Scalars['Boolean']['output']>;
};

export type Autoscaling = {
  __typename: 'Autoscaling';
  enabled: Maybe<Scalars['Boolean']['output']>;
  maxInstances: Maybe<Scalars['Int']['output']>;
  minInstances: Maybe<Scalars['Int']['output']>;
  targetCPUPercent: Maybe<Scalars['Int']['output']>;
  targetMemoryPercent: Maybe<Scalars['Int']['output']>;
};

export type Blueprint = {
  __typename: 'Blueprint';
  branch: Maybe<Scalars['String']['output']>;
  createdAt: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  manifest: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
  repo: Maybe<Scalars['String']['output']>;
  status: Maybe<Scalars['String']['output']>;
  updatedAt: Maybe<Scalars['String']['output']>;
};

export type BlueprintValidation = {
  __typename: 'BlueprintValidation';
  errorDetails: Maybe<Array<Maybe<BlueprintValidationError>>>;
  errors: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  plan: Maybe<BlueprintValidationPlan>;
  valid: Maybe<Scalars['Boolean']['output']>;
};

export type BlueprintValidationError = {
  __typename: 'BlueprintValidationError';
  column: Maybe<Scalars['Int']['output']>;
  error: Scalars['String']['output'];
  line: Maybe<Scalars['Int']['output']>;
  path: Maybe<Scalars['String']['output']>;
};

export type BlueprintValidationPlan = {
  __typename: 'BlueprintValidationPlan';
  databases: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  envGroups: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  keyValue: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  services: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  totalActions: Maybe<Scalars['Int']['output']>;
};

export type BuildFilter = {
  __typename: 'BuildFilter';
  ignoredPaths: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  paths: Maybe<Array<Maybe<Scalars['String']['output']>>>;
};

export type BuildFilterInput = {
  ignoredPaths?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  paths?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};

export type CronRun = {
  __typename: 'CronRun';
  finishedAt: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
  startedAt: Maybe<Scalars['String']['output']>;
  status: Maybe<Scalars['String']['output']>;
};

export type CustomDomain = {
  __typename: 'CustomDomain';
  dnsRecord: Maybe<DnsRecord>;
  domainType: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
  redirectForName: Maybe<Scalars['String']['output']>;
  serverStatus: Maybe<Scalars['String']['output']>;
  verificationStatus: Maybe<Scalars['String']['output']>;
};

export type DnsRecord = {
  __typename: 'DNSRecord';
  name: Maybe<Scalars['String']['output']>;
  type: Maybe<Scalars['String']['output']>;
  value: Maybe<Scalars['String']['output']>;
};

export type Database = {
  __typename: 'Database';
  backupsEnabled: Maybe<Scalars['Boolean']['output']>;
  createdAt: Maybe<Scalars['String']['output']>;
  dashboardUrl: Maybe<Scalars['String']['output']>;
  databaseName: Maybe<Scalars['String']['output']>;
  databaseUser: Maybe<Scalars['String']['output']>;
  diskAutoscalingEnabled: Maybe<Scalars['Boolean']['output']>;
  diskSizeGB: Maybe<Scalars['Int']['output']>;
  environmentId: Maybe<Scalars['String']['output']>;
  externalHost: Maybe<Scalars['String']['output']>;
  highAvailabilityEnabled: Maybe<Scalars['Boolean']['output']>;
  id: Maybe<Scalars['String']['output']>;
  ipAllowList: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  ipAllowListEntries: Maybe<Array<Maybe<IpAllowListEntry>>>;
  name: Maybe<Scalars['String']['output']>;
  ownerId: Maybe<Scalars['String']['output']>;
  plan: Maybe<Scalars['String']['output']>;
  poolerEnabled: Maybe<Scalars['Boolean']['output']>;
  projectId: Maybe<Scalars['String']['output']>;
  public: Maybe<Scalars['Boolean']['output']>;
  readReplicas: Maybe<Array<Maybe<ReadReplicaView>>>;
  region: Maybe<Scalars['String']['output']>;
  status: Maybe<Scalars['String']['output']>;
  suspended: Maybe<Scalars['String']['output']>;
  updatedAt: Maybe<Scalars['String']['output']>;
  version: Maybe<Scalars['String']['output']>;
};

export type DatabaseBackup = {
  __typename: 'DatabaseBackup';
  createdAt: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  status: Maybe<Scalars['String']['output']>;
};

export type DatabaseExport = {
  __typename: 'DatabaseExport';
  createdAt: Maybe<Scalars['String']['output']>;
  expiresAt: Maybe<Scalars['String']['output']>;
  failureReason: Maybe<Scalars['String']['output']>;
  filename: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  status: Maybe<Scalars['String']['output']>;
  url: Maybe<Scalars['String']['output']>;
  urlExpiresAt: Maybe<Scalars['String']['output']>;
};

export type DatabaseInstanceType = {
  __typename: 'DatabaseInstanceType';
  cpu: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  memory: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
  storageGB: Maybe<Scalars['Int']['output']>;
};

export type DatabaseLogEntry = {
  __typename: 'DatabaseLogEntry';
  message: Maybe<Scalars['String']['output']>;
  timestamp: Maybe<Scalars['String']['output']>;
};

export type DatabaseParameterOverride = {
  __typename: 'DatabaseParameterOverride';
  description: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
  setting: Maybe<Scalars['String']['output']>;
  source: Maybe<Scalars['String']['output']>;
  unit: Maybe<Scalars['String']['output']>;
};

export type DatabaseProcess = {
  __typename: 'DatabaseProcess';
  applicationName: Maybe<Scalars['String']['output']>;
  durationSeconds: Maybe<Scalars['Int']['output']>;
  pid: Maybe<Scalars['Int']['output']>;
  query: Maybe<Scalars['String']['output']>;
  state: Maybe<Scalars['String']['output']>;
  userName: Maybe<Scalars['String']['output']>;
  waitEvent: Maybe<Scalars['String']['output']>;
  waitEventType: Maybe<Scalars['String']['output']>;
};

export type DatabaseQueryResult = {
  __typename: 'DatabaseQueryResult';
  columns: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  rowCount: Maybe<Scalars['Int']['output']>;
  rows: Maybe<Array<Maybe<DatabaseQueryRow>>>;
  truncated: Maybe<Scalars['Boolean']['output']>;
};

export type DatabaseQueryRow = {
  __typename: 'DatabaseQueryRow';
  values: Maybe<Array<Maybe<Scalars['String']['output']>>>;
};

export type DatabaseRecoveryInfo = {
  __typename: 'DatabaseRecoveryInfo';
  backups: Maybe<Array<Maybe<DatabaseBackup>>>;
  earliestRecoveryTime: Maybe<Scalars['String']['output']>;
  enabled: Maybe<Scalars['Boolean']['output']>;
  latestRecoveryTime: Maybe<Scalars['String']['output']>;
};

export type DatabaseSizeInfo = {
  __typename: 'DatabaseSizeInfo';
  name: Maybe<Scalars['String']['output']>;
  sizeBytes: Maybe<Scalars['Int']['output']>;
  sizePretty: Maybe<Scalars['String']['output']>;
};

export type DatabaseSizes = {
  __typename: 'DatabaseSizes';
  database: Maybe<DatabaseSizeInfo>;
  tables: Maybe<Array<Maybe<TableSizeInfo>>>;
};

export type DatabaseTableScan = {
  __typename: 'DatabaseTableScan';
  deadRows: Maybe<Scalars['Int']['output']>;
  indexScanRows: Maybe<Scalars['Int']['output']>;
  indexScans: Maybe<Scalars['Int']['output']>;
  liveRows: Maybe<Scalars['Int']['output']>;
  name: Maybe<Scalars['String']['output']>;
  schema: Maybe<Scalars['String']['output']>;
  seqScanRows: Maybe<Scalars['Int']['output']>;
  seqScans: Maybe<Scalars['Int']['output']>;
};

export type DatabaseTopQuery = {
  __typename: 'DatabaseTopQuery';
  calls: Maybe<Scalars['Int']['output']>;
  meanTimeMs: Maybe<Scalars['Float']['output']>;
  query: Maybe<Scalars['String']['output']>;
  rows: Maybe<Scalars['Int']['output']>;
  sharedHitBlks: Maybe<Scalars['Int']['output']>;
  sharedReadBlks: Maybe<Scalars['Int']['output']>;
  totalTimeMs: Maybe<Scalars['Float']['output']>;
};

export type DatabaseUser = {
  __typename: 'DatabaseUser';
  name: Maybe<Scalars['String']['output']>;
};

export type DatabaseUserWithPassword = {
  __typename: 'DatabaseUserWithPassword';
  name: Maybe<Scalars['String']['output']>;
  password: Maybe<Scalars['String']['output']>;
};

export type DatastoreMetricsQueryInput = {
  end?: InputMaybe<Scalars['String']['input']>;
  kind?: InputMaybe<Scalars['String']['input']>;
  name: Scalars['String']['input'];
  resolution?: InputMaybe<Scalars['Int']['input']>;
  resource: Scalars['String']['input'];
  start?: InputMaybe<Scalars['String']['input']>;
};

export type Deploy = {
  __typename: 'Deploy';
  commitCreatedAt: Maybe<Scalars['String']['output']>;
  commitId: Maybe<Scalars['String']['output']>;
  commitMessage: Maybe<Scalars['String']['output']>;
  createdAt: Maybe<Scalars['String']['output']>;
  finishedAt: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  image: Maybe<Scalars['String']['output']>;
  preDeployStatus: Maybe<Scalars['String']['output']>;
  rollbackOf: Maybe<Scalars['String']['output']>;
  serviceId: Maybe<Scalars['String']['output']>;
  startedAt: Maybe<Scalars['String']['output']>;
  status: Maybe<Scalars['String']['output']>;
  trigger: Maybe<Scalars['String']['output']>;
  updatedAt: Maybe<Scalars['String']['output']>;
};

export type DeployHook = {
  __typename: 'DeployHook';
  url: Maybe<Scalars['String']['output']>;
};

export type DeployTrigger = {
  __typename: 'DeployTrigger';
  clearCache: Maybe<Scalars['Boolean']['output']>;
  deployedByRender: Maybe<Scalars['Boolean']['output']>;
  envUpdated: Maybe<Scalars['Boolean']['output']>;
  firstBuild: Maybe<Scalars['Boolean']['output']>;
  manual: Maybe<Scalars['Boolean']['output']>;
  rollback: Maybe<Scalars['Boolean']['output']>;
};

export type EnvGroup = {
  __typename: 'EnvGroup';
  createdAt: Maybe<Scalars['String']['output']>;
  envVars: Maybe<Array<Maybe<EnvGroupVar>>>;
  environmentId: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
  ownerId: Maybe<Scalars['String']['output']>;
  secretFiles: Maybe<Array<Maybe<EnvGroupSecretFile>>>;
  serviceLinks: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  updatedAt: Maybe<Scalars['String']['output']>;
};

export type EnvGroupSecretFile = {
  __typename: 'EnvGroupSecretFile';
  content: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
};

export type EnvGroupSecretFileInput = {
  content: Scalars['String']['input'];
  name: Scalars['String']['input'];
};

export type EnvGroupVar = {
  __typename: 'EnvGroupVar';
  key: Maybe<Scalars['String']['output']>;
  value: Maybe<Scalars['String']['output']>;
};

export type EnvGroupVarInput = {
  generateValue?: InputMaybe<Scalars['Boolean']['input']>;
  key: Scalars['String']['input'];
  value?: InputMaybe<Scalars['String']['input']>;
};

export type EnvVar = {
  __typename: 'EnvVar';
  id: Maybe<Scalars['String']['output']>;
  key: Maybe<Scalars['String']['output']>;
  value: Maybe<Scalars['String']['output']>;
};

export type EnvVarInput = {
  generateValue?: InputMaybe<Scalars['Boolean']['input']>;
  key: Scalars['String']['input'];
  value?: InputMaybe<Scalars['String']['input']>;
};

export type EnvVarListValue = {
  __typename: 'EnvVarListValue';
  id: Maybe<Scalars['String']['output']>;
  key: Maybe<Scalars['String']['output']>;
  value: Maybe<Scalars['String']['output']>;
};

export type EnvVarWithCursor = {
  __typename: 'EnvVarWithCursor';
  cursor: Maybe<Scalars['String']['output']>;
  envVar: Maybe<EnvVarListValue>;
};

export type Environment = {
  __typename: 'Environment';
  createdAt: Maybe<Scalars['String']['output']>;
  databaseIds: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  envGroupIds: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  id: Maybe<Scalars['String']['output']>;
  ipAllowList: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  ipAllowListEntries: Maybe<Array<Maybe<IpAllowListEntry>>>;
  keyValueIds: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  name: Maybe<Scalars['String']['output']>;
  networkIsolationEnabled: Maybe<Scalars['Boolean']['output']>;
  ownerId: Maybe<Scalars['String']['output']>;
  projectId: Maybe<Scalars['String']['output']>;
  protectedStatus: Maybe<Scalars['String']['output']>;
  serviceIds: Maybe<Array<Maybe<Scalars['String']['output']>>>;
};

export type EstimatedCost = {
  __typename: 'EstimatedCost';
  meters: Maybe<Array<Maybe<MeterEstimate>>>;
  totalUsd: Maybe<Scalars['String']['output']>;
};

export type GitConnection = {
  __typename: 'GitConnection';
  accountLogin: Maybe<Scalars['String']['output']>;
  connected: Maybe<Scalars['Boolean']['output']>;
  createdAt: Maybe<Scalars['String']['output']>;
  installUrl: Maybe<Scalars['String']['output']>;
  installationId: Maybe<Scalars['Float']['output']>;
};

export type IpAllowListEntry = {
  __typename: 'IPAllowListEntry';
  cidrBlock: Scalars['String']['output'];
  description: Maybe<Scalars['String']['output']>;
};

export type IpAllowListEntryInput = {
  cidrBlock: Scalars['String']['input'];
  description?: InputMaybe<Scalars['String']['input']>;
};

export type InstanceType = {
  __typename: 'InstanceType';
  cpu: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  memory: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
};

export type Job = {
  __typename: 'Job';
  createdAt: Maybe<Scalars['String']['output']>;
  finishedAt: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  planId: Maybe<Scalars['String']['output']>;
  serviceId: Maybe<Scalars['String']['output']>;
  startCommand: Maybe<Scalars['String']['output']>;
  startedAt: Maybe<Scalars['String']['output']>;
  status: Maybe<Scalars['String']['output']>;
};

export type KeyValue = {
  __typename: 'KeyValue';
  createdAt: Maybe<Scalars['String']['output']>;
  dashboardUrl: Maybe<Scalars['String']['output']>;
  environmentId: Maybe<Scalars['String']['output']>;
  externalHost: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  ipAllowList: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  ipAllowListEntries: Maybe<Array<Maybe<IpAllowListEntry>>>;
  maxmemoryPolicy: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
  ownerId: Maybe<Scalars['String']['output']>;
  persistenceMode: Maybe<Scalars['String']['output']>;
  plan: Maybe<Scalars['String']['output']>;
  projectId: Maybe<Scalars['String']['output']>;
  public: Maybe<Scalars['Boolean']['output']>;
  region: Maybe<Scalars['String']['output']>;
  status: Maybe<Scalars['String']['output']>;
  suspended: Maybe<Scalars['String']['output']>;
  updatedAt: Maybe<Scalars['String']['output']>;
  version: Maybe<Scalars['String']['output']>;
};

export type KeyValueConnectionInfo = {
  __typename: 'KeyValueConnectionInfo';
  cliCommand: Maybe<Scalars['String']['output']>;
  externalConnectionString: Maybe<Scalars['String']['output']>;
  internalConnectionString: Maybe<Scalars['String']['output']>;
};

export type KeyValueInstanceType = {
  __typename: 'KeyValueInstanceType';
  cpu: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  memory: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
  storageGB: Maybe<Scalars['Int']['output']>;
};

export type LogEntry = {
  __typename: 'LogEntry';
  instance: Maybe<Scalars['String']['output']>;
  level: Maybe<Scalars['String']['output']>;
  message: Maybe<Scalars['String']['output']>;
  method: Maybe<Scalars['String']['output']>;
  statusCode: Maybe<Scalars['String']['output']>;
  timestamp: Maybe<Scalars['String']['output']>;
  type: Maybe<Scalars['String']['output']>;
};

export type MaintenanceMode = {
  __typename: 'MaintenanceMode';
  enabled: Scalars['Boolean']['output'];
  uri: Scalars['String']['output'];
};

export type MaintenanceModeInput = {
  enabled: Scalars['Boolean']['input'];
  uri?: InputMaybe<Scalars['String']['input']>;
};

export type MeterEstimate = {
  __typename: 'MeterEstimate';
  costUsd: Maybe<Scalars['String']['output']>;
  kind: Maybe<Scalars['String']['output']>;
  resourceKind: Maybe<Scalars['String']['output']>;
  tier: Maybe<Scalars['String']['output']>;
};

export type MetricLabel = {
  __typename: 'MetricLabel';
  field: Maybe<Scalars['String']['output']>;
  value: Maybe<Scalars['String']['output']>;
};

export type MetricSeries = {
  __typename: 'MetricSeries';
  labels: Maybe<Array<Maybe<MetricLabel>>>;
  parameters: Maybe<Array<Maybe<MetricSeriesParameter>>>;
  unit: Maybe<Scalars['String']['output']>;
  values: Maybe<Array<Maybe<MetricValue>>>;
};

export type MetricSeriesParameter = {
  __typename: 'MetricSeriesParameter';
  quantile: Maybe<Scalars['Float']['output']>;
};

export type MetricValue = {
  __typename: 'MetricValue';
  time: Maybe<Scalars['String']['output']>;
  value: Maybe<Scalars['Float']['output']>;
};

export type MetricsFilterInput = {
  field: Scalars['String']['input'];
  values: Array<Scalars['String']['input']>;
};

export type MetricsFilterValues = {
  __typename: 'MetricsFilterValues';
  field: Maybe<Scalars['String']['output']>;
  values: Maybe<Array<Maybe<Scalars['String']['output']>>>;
};

export type MetricsFiltersQueryInput = {
  end?: InputMaybe<Scalars['String']['input']>;
  filters: Array<MetricsFilterInput>;
  outputFilters: Array<Scalars['String']['input']>;
  ownerId?: InputMaybe<Scalars['String']['input']>;
  start?: InputMaybe<Scalars['String']['input']>;
  type?: InputMaybe<Scalars['String']['input']>;
};

export type MetricsFiltersResult = {
  __typename: 'MetricsFiltersResult';
  values: Maybe<Array<Maybe<MetricsFilterValues>>>;
};

export type MetricsParameterInput = {
  quantile?: InputMaybe<Scalars['Float']['input']>;
};

export type MetricsPathFilterSuggestions = {
  __typename: 'MetricsPathFilterSuggestions';
  paths: Maybe<Array<Maybe<Scalars['String']['output']>>>;
};

export type MetricsPathFilterSuggestionsInput = {
  paths?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  serviceIDs: Array<Scalars['String']['input']>;
};

export type MetricsQueryInput = {
  aggregateAllMethod?: InputMaybe<Scalars['String']['input']>;
  aggregateBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  aggregationMethod?: InputMaybe<Scalars['String']['input']>;
  end?: InputMaybe<Scalars['String']['input']>;
  filters: Array<MetricsFilterInput>;
  name: Scalars['String']['input'];
  parameters?: InputMaybe<Array<InputMaybe<MetricsParameterInput>>>;
  resolution?: InputMaybe<Scalars['Int']['input']>;
  start?: InputMaybe<Scalars['String']['input']>;
};

export type MonthToDateBandwidth = {
  __typename: 'MonthToDateBandwidth';
  egressBandwidthMB: Maybe<Scalars['Float']['output']>;
  httpEgressBandwidthMB: Maybe<Scalars['Float']['output']>;
  natEgressBandwidthMB: Maybe<Scalars['Float']['output']>;
  privateLinkEgressBandwidthMB: Maybe<Scalars['Float']['output']>;
  websocketEgressBandwidthMB: Maybe<Scalars['Float']['output']>;
};

export type Mutation = {
  __typename: 'Mutation';
  acceptWorkspaceInvite: Maybe<AcceptedWorkspaceInvite>;
  addCustomDomain: Maybe<CustomDomain>;
  cancelCronJobRun: Maybe<CronRun>;
  cancelDeploy: Maybe<Deploy>;
  cancelJob: Maybe<Job>;
  changeWorkspaceMemberRole: Maybe<WorkspaceMember>;
  changeWorkspacePlan: Maybe<Workspace>;
  connectGit: Maybe<GitConnection>;
  createApiKey: Maybe<ApiKey>;
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
  createWebhookEndpoint: Maybe<WebhookEndpoint>;
  createWorkspace: Maybe<Workspace>;
  deleteCustomDomain: Maybe<Scalars['Boolean']['output']>;
  deleteDatabase: Maybe<Scalars['Boolean']['output']>;
  deleteDatabaseUser: Maybe<Scalars['Boolean']['output']>;
  deleteEnvGroup: Maybe<Scalars['Boolean']['output']>;
  deleteEnvGroupSecretFile: Maybe<Scalars['Boolean']['output']>;
  deleteEnvGroupVar: Maybe<Scalars['Boolean']['output']>;
  deleteEnvVar: Maybe<Scalars['Boolean']['output']>;
  deleteEnvironment: Maybe<Scalars['String']['output']>;
  deleteKeyValue: Maybe<Scalars['Boolean']['output']>;
  deleteProject: Maybe<Scalars['String']['output']>;
  deleteRegistryCredential: Maybe<Scalars['Boolean']['output']>;
  deleteSSHKey: Scalars['Boolean']['output'];
  deleteSecretFile: Maybe<Scalars['Boolean']['output']>;
  deleteService: Maybe<Scalars['Boolean']['output']>;
  deleteWebhookEndpoint: Maybe<Scalars['Boolean']['output']>;
  deleteWorkspace: Maybe<Scalars['String']['output']>;
  disableAutoscaling: Maybe<Scalars['Boolean']['output']>;
  disconnectGit: Maybe<Scalars['Boolean']['output']>;
  executeDatabaseQuery: Maybe<DatabaseQueryResult>;
  failoverDatabase: Maybe<Scalars['Boolean']['output']>;
  inviteWorkspaceMember: Maybe<WorkspaceInvite>;
  linkEnvGroup: Maybe<Scalars['Boolean']['output']>;
  recoverDatabase: Maybe<Database>;
  regenerateDeployHook: Maybe<DeployHook>;
  removeWorkspaceMember: Maybe<Scalars['String']['output']>;
  renameDatabase: Maybe<Database>;
  renameEnvGroup: Maybe<EnvGroup>;
  renameEnvironment: Maybe<Environment>;
  renameKeyValue: Maybe<KeyValue>;
  renameProject: Maybe<Project>;
  renameWorkspace: Maybe<Workspace>;
  resendWorkspaceInvite: Maybe<WorkspaceInvite>;
  restartDatabase: Maybe<Database>;
  restartServer: Maybe<Deploy>;
  resumeDatabase: Maybe<Database>;
  resumeKeyValue: Maybe<KeyValue>;
  resumeService: Maybe<Service>;
  revokeApiKey: Maybe<Scalars['Boolean']['output']>;
  revokeWorkspaceInvite: Maybe<Scalars['String']['output']>;
  rollbackService: Maybe<Deploy>;
  runCronJob: Maybe<CronRun>;
  scaleService: Maybe<Service>;
  setAutoDeploy: Maybe<Service>;
  setAutoscaling: Maybe<Autoscaling>;
  setBuildCommand: Maybe<Service>;
  setBuildFilter: Maybe<Service>;
  setDatabaseIpAllowList: Maybe<Database>;
  setDatabaseParameterOverrides: Maybe<Database>;
  setDisplayName: Maybe<Service>;
  setDockerfilePath: Maybe<Service>;
  setEnvGroupSecretFile: Maybe<Scalars['Boolean']['output']>;
  setEnvGroupVar: Maybe<Scalars['Boolean']['output']>;
  setEnvGroupVars: Maybe<Scalars['Boolean']['output']>;
  setEnvVar: Maybe<Scalars['Boolean']['output']>;
  setEnvVars: Maybe<Scalars['Boolean']['output']>;
  setEnvironmentACL: Maybe<Environment>;
  setEnvironmentDatabases: Maybe<Environment>;
  setEnvironmentEnvGroups: Maybe<Environment>;
  setEnvironmentKeyValues: Maybe<Environment>;
  setEnvironmentServices: Maybe<Environment>;
  setHealthCheckPath: Maybe<Service>;
  setIdleTimeout: Maybe<Service>;
  setKeyValueIpAllowList: Maybe<KeyValue>;
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
  setRootDir: Maybe<Service>;
  setSecretFile: Maybe<Scalars['Boolean']['output']>;
  setServiceIpAllowList: Maybe<Service>;
  setStartCommand: Maybe<Service>;
  setStaticHeaders: Maybe<Service>;
  setStaticRoutes: Maybe<Service>;
  setSubdomainPolicy: Maybe<Service>;
  setWebhookEndpointEnabled: Maybe<WebhookEndpoint>;
  suspendDatabase: Maybe<Database>;
  suspendKeyValue: Maybe<KeyValue>;
  suspendService: Maybe<Service>;
  syncBlueprint: Maybe<SyncBlueprintResult>;
  triggerDeploy: Maybe<Deploy>;
  unlinkEnvGroup: Maybe<Scalars['Boolean']['output']>;
  updateCronJob: Maybe<Service>;
  updateDatabaseDiskAutoscaling: Maybe<Database>;
  updateDatabasePlan: Maybe<Database>;
  updateDatabaseVersion: Maybe<Database>;
  updateEnvironment: Maybe<Environment>;
  updateKeyValuePlan: Maybe<KeyValue>;
  updateNotificationSettings: Maybe<NotificationSettings>;
  updateRegistryCredential: Maybe<RegistryCredential>;
  updateServicePlan: Maybe<Service>;
  updateWebhookEndpoint: Maybe<WebhookEndpoint>;
  verifyCustomDomain: Maybe<CustomDomain>;
};


export type MutationAcceptWorkspaceInviteArgs = {
  token: Scalars['String']['input'];
};


export type MutationAddCustomDomainArgs = {
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
};


export type MutationCancelCronJobRunArgs = {
  runId: Scalars['String']['input'];
  serviceId: Scalars['String']['input'];
};


export type MutationCancelDeployArgs = {
  deployId: Scalars['String']['input'];
  serviceId: Scalars['String']['input'];
};


export type MutationCancelJobArgs = {
  jobId: Scalars['String']['input'];
  serviceId: Scalars['String']['input'];
};


export type MutationChangeWorkspaceMemberRoleArgs = {
  role: Scalars['String']['input'];
  subject: Scalars['String']['input'];
  workspaceId: Scalars['String']['input'];
};


export type MutationChangeWorkspacePlanArgs = {
  id: Scalars['String']['input'];
  plan: Scalars['String']['input'];
};


export type MutationConnectGitArgs = {
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type MutationCreateApiKeyArgs = {
  name: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type MutationCreateDatabaseArgs = {
  diskSizeGB?: InputMaybe<Scalars['Int']['input']>;
  dryRun?: InputMaybe<Scalars['Boolean']['input']>;
  enableDiskAutoscaling?: InputMaybe<Scalars['Boolean']['input']>;
  enableHighAvailability?: InputMaybe<Scalars['Boolean']['input']>;
  environmentId?: InputMaybe<Scalars['String']['input']>;
  ipAllowList?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  ipAllowListEntries?: InputMaybe<Array<IpAllowListEntryInput>>;
  name: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
  plan?: InputMaybe<Scalars['String']['input']>;
  public?: InputMaybe<Scalars['Boolean']['input']>;
  version?: InputMaybe<Scalars['String']['input']>;
};


export type MutationCreateDatabaseExportArgs = {
  id: Scalars['String']['input'];
};


export type MutationCreateDatabaseUserArgs = {
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
};


export type MutationCreateEnvGroupArgs = {
  envVars?: InputMaybe<Array<EnvGroupVarInput>>;
  environmentId?: InputMaybe<Scalars['String']['input']>;
  name: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
  secretFiles?: InputMaybe<Array<EnvGroupSecretFileInput>>;
  serviceIds?: InputMaybe<Array<Scalars['String']['input']>>;
};


export type MutationCreateEnvironmentArgs = {
  ipAllowList?: InputMaybe<Array<IpAllowListEntryInput>>;
  name: Scalars['String']['input'];
  networkIsolationEnabled?: InputMaybe<Scalars['Boolean']['input']>;
  projectId: Scalars['String']['input'];
  protectedStatus?: InputMaybe<Scalars['String']['input']>;
};


export type MutationCreateJobArgs = {
  planId?: InputMaybe<Scalars['String']['input']>;
  serviceId: Scalars['String']['input'];
  startCommand: Scalars['String']['input'];
};


export type MutationCreateKeyValueArgs = {
  dryRun?: InputMaybe<Scalars['Boolean']['input']>;
  environmentId?: InputMaybe<Scalars['String']['input']>;
  ipAllowList?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  ipAllowListEntries?: InputMaybe<Array<IpAllowListEntryInput>>;
  maxmemoryPolicy?: InputMaybe<Scalars['String']['input']>;
  name: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
  persistenceMode?: InputMaybe<Scalars['String']['input']>;
  plan?: InputMaybe<Scalars['String']['input']>;
  public?: InputMaybe<Scalars['Boolean']['input']>;
  storageGB?: InputMaybe<Scalars['Int']['input']>;
  version?: InputMaybe<Scalars['String']['input']>;
};


export type MutationCreateProjectArgs = {
  name: Scalars['String']['input'];
  ownerId: Scalars['String']['input'];
};


export type MutationCreateRegistryCredentialArgs = {
  authToken: Scalars['String']['input'];
  expiresAt?: InputMaybe<Scalars['String']['input']>;
  host: Scalars['String']['input'];
  name?: InputMaybe<Scalars['String']['input']>;
  ownerId?: InputMaybe<Scalars['String']['input']>;
  username: Scalars['String']['input'];
};


export type MutationCreateSshKeyArgs = {
  name: Scalars['String']['input'];
  publicKey: Scalars['String']['input'];
};


export type MutationCreateServiceArgs = {
  autoDeploy?: InputMaybe<Scalars['Boolean']['input']>;
  branch?: InputMaybe<Scalars['String']['input']>;
  buildCommand?: InputMaybe<Scalars['String']['input']>;
  buildFilter?: InputMaybe<BuildFilterInput>;
  builder?: InputMaybe<Scalars['String']['input']>;
  command?: InputMaybe<Scalars['String']['input']>;
  dockerfilePath?: InputMaybe<Scalars['String']['input']>;
  dryRun?: InputMaybe<Scalars['Boolean']['input']>;
  envVars?: InputMaybe<Array<InputMaybe<EnvVarInput>>>;
  environmentId?: InputMaybe<Scalars['String']['input']>;
  headers?: InputMaybe<Array<InputMaybe<StaticHeaderInput>>>;
  healthCheckPath?: InputMaybe<Scalars['String']['input']>;
  image?: InputMaybe<Scalars['String']['input']>;
  maintenanceMode?: InputMaybe<MaintenanceModeInput>;
  maxShutdownDelaySeconds?: InputMaybe<Scalars['Int']['input']>;
  name: Scalars['String']['input'];
  notifyOnFail?: InputMaybe<Scalars['String']['input']>;
  ownerId?: InputMaybe<Scalars['String']['input']>;
  plan?: InputMaybe<Scalars['String']['input']>;
  port?: InputMaybe<Scalars['Int']['input']>;
  preDeployCommand?: InputMaybe<Scalars['String']['input']>;
  publishPath?: InputMaybe<Scalars['String']['input']>;
  registryCredentialId?: InputMaybe<Scalars['String']['input']>;
  replicas?: InputMaybe<Scalars['Int']['input']>;
  repo?: InputMaybe<Scalars['String']['input']>;
  rootDir?: InputMaybe<Scalars['String']['input']>;
  routes?: InputMaybe<Array<InputMaybe<StaticRouteInput>>>;
  runtime?: InputMaybe<Scalars['String']['input']>;
  schedule?: InputMaybe<Scalars['String']['input']>;
  secretFiles?: InputMaybe<Array<InputMaybe<SecretFileInput>>>;
  startCommand?: InputMaybe<Scalars['String']['input']>;
  type?: InputMaybe<Scalars['String']['input']>;
};


export type MutationCreateWebhookEndpointArgs = {
  eventTypes: Array<InputMaybe<Scalars['String']['input']>>;
  name?: InputMaybe<Scalars['String']['input']>;
  ownerId?: InputMaybe<Scalars['String']['input']>;
  url: Scalars['String']['input'];
};


export type MutationCreateWorkspaceArgs = {
  name: Scalars['String']['input'];
  plan?: InputMaybe<Scalars['String']['input']>;
};


export type MutationDeleteCustomDomainArgs = {
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
};


export type MutationDeleteDatabaseArgs = {
  id: Scalars['String']['input'];
};


export type MutationDeleteDatabaseUserArgs = {
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
};


export type MutationDeleteEnvGroupArgs = {
  id: Scalars['String']['input'];
};


export type MutationDeleteEnvGroupSecretFileArgs = {
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
};


export type MutationDeleteEnvGroupVarArgs = {
  id: Scalars['String']['input'];
  key: Scalars['String']['input'];
};


export type MutationDeleteEnvVarArgs = {
  key: Scalars['String']['input'];
  serviceId: Scalars['String']['input'];
};


export type MutationDeleteEnvironmentArgs = {
  id: Scalars['String']['input'];
};


export type MutationDeleteKeyValueArgs = {
  id: Scalars['String']['input'];
};


export type MutationDeleteProjectArgs = {
  id: Scalars['String']['input'];
};


export type MutationDeleteRegistryCredentialArgs = {
  id: Scalars['String']['input'];
};


export type MutationDeleteSshKeyArgs = {
  id: Scalars['String']['input'];
};


export type MutationDeleteSecretFileArgs = {
  name: Scalars['String']['input'];
  serviceId: Scalars['String']['input'];
};


export type MutationDeleteServiceArgs = {
  confirm?: InputMaybe<Scalars['String']['input']>;
  id: Scalars['String']['input'];
};


export type MutationDeleteWebhookEndpointArgs = {
  id: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type MutationDeleteWorkspaceArgs = {
  confirmation: Scalars['String']['input'];
  id: Scalars['String']['input'];
};


export type MutationDisableAutoscalingArgs = {
  id: Scalars['String']['input'];
};


export type MutationDisconnectGitArgs = {
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type MutationExecuteDatabaseQueryArgs = {
  allowWrites?: InputMaybe<Scalars['Boolean']['input']>;
  id: Scalars['String']['input'];
  sql: Scalars['String']['input'];
};


export type MutationFailoverDatabaseArgs = {
  id: Scalars['String']['input'];
};


export type MutationInviteWorkspaceMemberArgs = {
  email: Scalars['String']['input'];
  role: Scalars['String']['input'];
  workspaceId: Scalars['String']['input'];
};


export type MutationLinkEnvGroupArgs = {
  id: Scalars['String']['input'];
  serviceId: Scalars['String']['input'];
};


export type MutationRecoverDatabaseArgs = {
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
  plan?: InputMaybe<Scalars['String']['input']>;
  targetTime?: InputMaybe<Scalars['String']['input']>;
  version?: InputMaybe<Scalars['String']['input']>;
};


export type MutationRegenerateDeployHookArgs = {
  serviceId: Scalars['String']['input'];
};


export type MutationRemoveWorkspaceMemberArgs = {
  subject: Scalars['String']['input'];
  workspaceId: Scalars['String']['input'];
};


export type MutationRenameDatabaseArgs = {
  dryRun?: InputMaybe<Scalars['Boolean']['input']>;
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
};


export type MutationRenameEnvGroupArgs = {
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
};


export type MutationRenameEnvironmentArgs = {
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
};


export type MutationRenameKeyValueArgs = {
  dryRun?: InputMaybe<Scalars['Boolean']['input']>;
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
};


export type MutationRenameProjectArgs = {
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
};


export type MutationRenameWorkspaceArgs = {
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
};


export type MutationResendWorkspaceInviteArgs = {
  inviteId: Scalars['String']['input'];
  workspaceId: Scalars['String']['input'];
};


export type MutationRestartDatabaseArgs = {
  id: Scalars['String']['input'];
};


export type MutationRestartServerArgs = {
  serviceId: Scalars['String']['input'];
};


export type MutationResumeDatabaseArgs = {
  id: Scalars['String']['input'];
};


export type MutationResumeKeyValueArgs = {
  id: Scalars['String']['input'];
};


export type MutationResumeServiceArgs = {
  id: Scalars['String']['input'];
};


export type MutationRevokeApiKeyArgs = {
  id: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type MutationRevokeWorkspaceInviteArgs = {
  inviteId: Scalars['String']['input'];
  workspaceId: Scalars['String']['input'];
};


export type MutationRollbackServiceArgs = {
  deployId: Scalars['String']['input'];
  serviceId: Scalars['String']['input'];
};


export type MutationRunCronJobArgs = {
  id: Scalars['String']['input'];
};


export type MutationScaleServiceArgs = {
  id: Scalars['String']['input'];
  numInstances: Scalars['Int']['input'];
};


export type MutationSetAutoDeployArgs = {
  enabled: Scalars['Boolean']['input'];
  id: Scalars['String']['input'];
};


export type MutationSetAutoscalingArgs = {
  id: Scalars['String']['input'];
  maxInstances: Scalars['Int']['input'];
  minInstances: Scalars['Int']['input'];
  targetCPUPercent?: InputMaybe<Scalars['Int']['input']>;
  targetMemoryPercent?: InputMaybe<Scalars['Int']['input']>;
};


export type MutationSetBuildCommandArgs = {
  command: Scalars['String']['input'];
  id: Scalars['String']['input'];
};


export type MutationSetBuildFilterArgs = {
  buildFilter: BuildFilterInput;
  id: Scalars['String']['input'];
};


export type MutationSetDatabaseIpAllowListArgs = {
  cidrs?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  entries?: InputMaybe<Array<IpAllowListEntryInput>>;
  id: Scalars['String']['input'];
};


export type MutationSetDatabaseParameterOverridesArgs = {
  id: Scalars['String']['input'];
  parameters?: InputMaybe<Array<InputMaybe<ParameterInput>>>;
};


export type MutationSetDisplayNameArgs = {
  displayName: Scalars['String']['input'];
  id: Scalars['String']['input'];
};


export type MutationSetDockerfilePathArgs = {
  dockerfilePath: Scalars['String']['input'];
  id: Scalars['String']['input'];
};


export type MutationSetEnvGroupSecretFileArgs = {
  content?: InputMaybe<Scalars['String']['input']>;
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
};


export type MutationSetEnvGroupVarArgs = {
  id: Scalars['String']['input'];
  key: Scalars['String']['input'];
  value?: InputMaybe<Scalars['String']['input']>;
};


export type MutationSetEnvGroupVarsArgs = {
  envVars: Array<EnvGroupVarInput>;
  id: Scalars['String']['input'];
};


export type MutationSetEnvVarArgs = {
  generateValue?: InputMaybe<Scalars['Boolean']['input']>;
  key: Scalars['String']['input'];
  serviceId: Scalars['String']['input'];
  value?: InputMaybe<Scalars['String']['input']>;
};


export type MutationSetEnvVarsArgs = {
  envVars: Array<EnvVarInput>;
  serviceId: Scalars['String']['input'];
};


export type MutationSetEnvironmentAclArgs = {
  id: Scalars['String']['input'];
  ipAllowList?: InputMaybe<Array<Scalars['String']['input']>>;
  ipAllowListEntries?: InputMaybe<Array<IpAllowListEntryInput>>;
  networkIsolationEnabled: Scalars['Boolean']['input'];
  protectedStatus: Scalars['String']['input'];
};


export type MutationSetEnvironmentDatabasesArgs = {
  databaseIds: Array<Scalars['String']['input']>;
  id: Scalars['String']['input'];
};


export type MutationSetEnvironmentEnvGroupsArgs = {
  envGroupIds: Array<Scalars['String']['input']>;
  id: Scalars['String']['input'];
};


export type MutationSetEnvironmentKeyValuesArgs = {
  id: Scalars['String']['input'];
  keyValueIds: Array<Scalars['String']['input']>;
};


export type MutationSetEnvironmentServicesArgs = {
  id: Scalars['String']['input'];
  serviceIds: Array<Scalars['String']['input']>;
};


export type MutationSetHealthCheckPathArgs = {
  id: Scalars['String']['input'];
  path: Scalars['String']['input'];
};


export type MutationSetIdleTimeoutArgs = {
  id: Scalars['String']['input'];
  idleTTLSeconds: Scalars['Int']['input'];
};


export type MutationSetKeyValueIpAllowListArgs = {
  cidrs?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  entries?: InputMaybe<Array<IpAllowListEntryInput>>;
  id: Scalars['String']['input'];
};


export type MutationSetMaintenanceModeArgs = {
  id: Scalars['String']['input'];
  maintenanceMode: MaintenanceModeInput;
};


export type MutationSetMaxShutdownDelayArgs = {
  id: Scalars['String']['input'];
  seconds: Scalars['Int']['input'];
};


export type MutationSetNotificationsToSendArgs = {
  id: Scalars['String']['input'];
  value: Scalars['String']['input'];
};


export type MutationSetNotifyOnFailArgs = {
  id: Scalars['String']['input'];
  value: Scalars['String']['input'];
};


export type MutationSetPreDeployCommandArgs = {
  command: Scalars['String']['input'];
  id: Scalars['String']['input'];
};


export type MutationSetProjectDatabasesArgs = {
  databaseIds: Array<Scalars['String']['input']>;
  id: Scalars['String']['input'];
};


export type MutationSetProjectKeyValuesArgs = {
  id: Scalars['String']['input'];
  keyValueIds: Array<Scalars['String']['input']>;
};


export type MutationSetProjectServicesArgs = {
  id: Scalars['String']['input'];
  serviceIds: Array<Scalars['String']['input']>;
};


export type MutationSetPublishPathArgs = {
  id: Scalars['String']['input'];
  publishPath: Scalars['String']['input'];
};


export type MutationSetRegistryCredentialArgs = {
  id: Scalars['String']['input'];
  registryCredentialId: Scalars['String']['input'];
};


export type MutationSetRootDirArgs = {
  id: Scalars['String']['input'];
  rootDir: Scalars['String']['input'];
};


export type MutationSetSecretFileArgs = {
  content?: InputMaybe<Scalars['String']['input']>;
  name: Scalars['String']['input'];
  serviceId: Scalars['String']['input'];
};


export type MutationSetServiceIpAllowListArgs = {
  cidrs?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  id: Scalars['String']['input'];
};


export type MutationSetStartCommandArgs = {
  command: Scalars['String']['input'];
  id: Scalars['String']['input'];
};


export type MutationSetStaticHeadersArgs = {
  headers?: InputMaybe<Array<InputMaybe<StaticHeaderInput>>>;
  id: Scalars['String']['input'];
};


export type MutationSetStaticRoutesArgs = {
  id: Scalars['String']['input'];
  routes?: InputMaybe<Array<InputMaybe<StaticRouteInput>>>;
};


export type MutationSetSubdomainPolicyArgs = {
  id: Scalars['String']['input'];
  policy: Scalars['String']['input'];
};


export type MutationSetWebhookEndpointEnabledArgs = {
  enabled: Scalars['Boolean']['input'];
  id: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type MutationSuspendDatabaseArgs = {
  id: Scalars['String']['input'];
};


export type MutationSuspendKeyValueArgs = {
  id: Scalars['String']['input'];
};


export type MutationSuspendServiceArgs = {
  confirm?: InputMaybe<Scalars['String']['input']>;
  id: Scalars['String']['input'];
};


export type MutationSyncBlueprintArgs = {
  bexYaml?: InputMaybe<Scalars['String']['input']>;
  confirm?: InputMaybe<Scalars['String']['input']>;
  id: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type MutationTriggerDeployArgs = {
  commitId?: InputMaybe<Scalars['String']['input']>;
  deployMode?: InputMaybe<Scalars['String']['input']>;
  imageUrl?: InputMaybe<Scalars['String']['input']>;
  serviceId: Scalars['String']['input'];
};


export type MutationUnlinkEnvGroupArgs = {
  id: Scalars['String']['input'];
  serviceId: Scalars['String']['input'];
};


export type MutationUpdateCronJobArgs = {
  command?: InputMaybe<Scalars['String']['input']>;
  id: Scalars['String']['input'];
  schedule: Scalars['String']['input'];
};


export type MutationUpdateDatabaseDiskAutoscalingArgs = {
  enabled: Scalars['Boolean']['input'];
  id: Scalars['String']['input'];
};


export type MutationUpdateDatabasePlanArgs = {
  dryRun?: InputMaybe<Scalars['Boolean']['input']>;
  id: Scalars['String']['input'];
  plan: Scalars['String']['input'];
};


export type MutationUpdateDatabaseVersionArgs = {
  id: Scalars['String']['input'];
  version: Scalars['String']['input'];
};


export type MutationUpdateEnvironmentArgs = {
  id: Scalars['String']['input'];
  ipAllowList?: InputMaybe<Array<Scalars['String']['input']>>;
  ipAllowListEntries?: InputMaybe<Array<IpAllowListEntryInput>>;
  name?: InputMaybe<Scalars['String']['input']>;
  networkIsolationEnabled?: InputMaybe<Scalars['Boolean']['input']>;
  protectedStatus?: InputMaybe<Scalars['String']['input']>;
};


export type MutationUpdateKeyValuePlanArgs = {
  dryRun?: InputMaybe<Scalars['Boolean']['input']>;
  id: Scalars['String']['input'];
  plan: Scalars['String']['input'];
};


export type MutationUpdateNotificationSettingsArgs = {
  deployFailed: Scalars['Boolean']['input'];
  deployStarted: Scalars['Boolean']['input'];
  deploySucceeded: Scalars['Boolean']['input'];
};


export type MutationUpdateRegistryCredentialArgs = {
  authToken?: InputMaybe<Scalars['String']['input']>;
  expiresAt?: InputMaybe<Scalars['String']['input']>;
  id: Scalars['String']['input'];
  name?: InputMaybe<Scalars['String']['input']>;
  username?: InputMaybe<Scalars['String']['input']>;
};


export type MutationUpdateServicePlanArgs = {
  dryRun?: InputMaybe<Scalars['Boolean']['input']>;
  id: Scalars['String']['input'];
  plan: Scalars['String']['input'];
};


export type MutationUpdateWebhookEndpointArgs = {
  enabled?: InputMaybe<Scalars['Boolean']['input']>;
  eventFilter?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  eventTypes?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  id: Scalars['String']['input'];
  name?: InputMaybe<Scalars['String']['input']>;
  ownerId?: InputMaybe<Scalars['String']['input']>;
  url?: InputMaybe<Scalars['String']['input']>;
};


export type MutationVerifyCustomDomainArgs = {
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
};

export type NameAvailability = {
  __typename: 'NameAvailability';
  available: Maybe<Scalars['Boolean']['output']>;
  suggestion: Maybe<Scalars['String']['output']>;
};

export type NotificationSettings = {
  __typename: 'NotificationSettings';
  deployFailed: Maybe<Scalars['Boolean']['output']>;
  deployStarted: Maybe<Scalars['Boolean']['output']>;
  deploySucceeded: Maybe<Scalars['Boolean']['output']>;
};

export type ParameterInput = {
  name: Scalars['String']['input'];
  value: Scalars['String']['input'];
};

export type PostgresConnectionInfo = {
  __typename: 'PostgresConnectionInfo';
  externalConnectionPoolString: Maybe<Scalars['String']['output']>;
  externalConnectionString: Maybe<Scalars['String']['output']>;
  internalConnectionPoolString: Maybe<Scalars['String']['output']>;
  internalConnectionString: Maybe<Scalars['String']['output']>;
  password: Maybe<Scalars['String']['output']>;
  psqlCommand: Maybe<Scalars['String']['output']>;
  readReplicaConnectionStrings: Maybe<Array<Maybe<ReplicaConnectionStrings>>>;
};

export type Project = {
  __typename: 'Project';
  createdAt: Maybe<Scalars['String']['output']>;
  databaseIds: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  id: Maybe<Scalars['String']['output']>;
  keyValueIds: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  name: Maybe<Scalars['String']['output']>;
  ownerId: Maybe<Scalars['String']['output']>;
  serviceIds: Maybe<Array<Maybe<Scalars['String']['output']>>>;
};

export type Query = {
  __typename: 'Query';
  apiKeys: Maybe<Array<Maybe<ApiKey>>>;
  auditLogs: Maybe<Array<Maybe<AuditLog>>>;
  autoscalingConfig: Maybe<Autoscaling>;
  blueprint: Maybe<Blueprint>;
  blueprints: Maybe<Array<Maybe<Blueprint>>>;
  cronJobRun: Maybe<CronRun>;
  cronJobRuns: Maybe<Array<Maybe<CronRun>>>;
  customDomain: Maybe<CustomDomain>;
  customDomains: Maybe<Array<Maybe<CustomDomain>>>;
  database: Maybe<Database>;
  databaseConnectionInfo: Maybe<PostgresConnectionInfo>;
  databaseExports: Maybe<Array<Maybe<DatabaseExport>>>;
  databaseInstanceTypes: Maybe<Array<Maybe<DatabaseInstanceType>>>;
  databaseIpAllowList: Maybe<Array<Maybe<Scalars['String']['output']>>>;
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
  gitConnection: Maybe<GitConnection>;
  instanceTypes: Maybe<Array<Maybe<InstanceType>>>;
  job: Maybe<Job>;
  jobs: Maybe<Array<Maybe<Job>>>;
  keyValue: Maybe<KeyValue>;
  keyValueConnectionInfo: Maybe<KeyValueConnectionInfo>;
  keyValueInstanceTypes: Maybe<Array<Maybe<KeyValueInstanceType>>>;
  keyValueIpAllowList: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  keyValues: Maybe<Array<Maybe<KeyValue>>>;
  logLabelValues: Maybe<Array<Scalars['String']['output']>>;
  logs: Maybe<Array<Maybe<LogEntry>>>;
  metrics: Maybe<Array<Maybe<MetricSeries>>>;
  metricsFilters: Maybe<MetricsFiltersResult>;
  metricsPathFilterSuggestions: Maybe<MetricsPathFilterSuggestions>;
  monthToDateBandwidth: Maybe<MonthToDateBandwidth>;
  notificationSettings: Maybe<NotificationSettings>;
  project: Maybe<Project>;
  projects: Maybe<Array<Maybe<Project>>>;
  registryCredential: Maybe<RegistryCredential>;
  registryCredentials: Maybe<Array<Maybe<RegistryCredential>>>;
  repos: Maybe<Array<Maybe<Repo>>>;
  secretFiles: Maybe<Array<Maybe<SecretFileWithCursor>>>;
  server: Maybe<Service>;
  service: Maybe<Service>;
  serviceEvents: Maybe<Array<Maybe<ServiceEvent>>>;
  serviceNameAvailable: Maybe<NameAvailability>;
  services: Maybe<Array<Maybe<Service>>>;
  sshKeys: Maybe<Array<Maybe<SshKey>>>;
  usage: Maybe<UsageSummary>;
  validateBlueprint: Maybe<BlueprintValidation>;
  webhookDeliveries: Maybe<Array<Maybe<WebhookDelivery>>>;
  webhookEndpoint: Maybe<WebhookEndpoint>;
  webhookEndpoints: Maybe<Array<Maybe<WebhookEndpoint>>>;
  webhookEventTypes: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  workspaceInvites: Maybe<Array<Maybe<WorkspaceInvite>>>;
  workspaceLimits: Maybe<ResourceLimits>;
  workspaceMembers: Maybe<Array<Maybe<WorkspaceMember>>>;
  workspaceSeatUsage: Maybe<WorkspaceSeatUsage>;
  workspaces: Maybe<Array<Maybe<Workspace>>>;
};


export type QueryApiKeysArgs = {
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type QueryAuditLogsArgs = {
  cursor?: InputMaybe<Scalars['String']['input']>;
  direction?: InputMaybe<Scalars['String']['input']>;
  endTime?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  ownerId: Scalars['String']['input'];
  startTime?: InputMaybe<Scalars['String']['input']>;
};


export type QueryAutoscalingConfigArgs = {
  id: Scalars['String']['input'];
};


export type QueryBlueprintArgs = {
  id: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type QueryBlueprintsArgs = {
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type QueryCronJobRunArgs = {
  runId: Scalars['String']['input'];
  serviceId: Scalars['String']['input'];
};


export type QueryCronJobRunsArgs = {
  cursor?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  serviceId: Scalars['String']['input'];
};


export type QueryCustomDomainArgs = {
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
};


export type QueryCustomDomainsArgs = {
  id: Scalars['String']['input'];
};


export type QueryDatabaseArgs = {
  id: Scalars['String']['input'];
};


export type QueryDatabaseConnectionInfoArgs = {
  id: Scalars['String']['input'];
};


export type QueryDatabaseExportsArgs = {
  id: Scalars['String']['input'];
};


export type QueryDatabaseIpAllowListArgs = {
  id: Scalars['String']['input'];
};


export type QueryDatabaseLogsArgs = {
  direction?: InputMaybe<Scalars['String']['input']>;
  endTime?: InputMaybe<Scalars['String']['input']>;
  id: Scalars['String']['input'];
  instance?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  startTime?: InputMaybe<Scalars['String']['input']>;
  text?: InputMaybe<Scalars['String']['input']>;
};


export type QueryDatabaseParameterOverridesArgs = {
  id: Scalars['String']['input'];
};


export type QueryDatabaseProcessesArgs = {
  id: Scalars['String']['input'];
};


export type QueryDatabaseRecoveryInfoArgs = {
  id: Scalars['String']['input'];
};


export type QueryDatabaseSizesArgs = {
  id: Scalars['String']['input'];
};


export type QueryDatabaseTableScansArgs = {
  id: Scalars['String']['input'];
};


export type QueryDatabaseTopQueriesArgs = {
  id: Scalars['String']['input'];
};


export type QueryDatabaseUsersArgs = {
  id: Scalars['String']['input'];
};


export type QueryDatabasesArgs = {
  cursor?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type QueryDatastoreMetricsArgs = {
  query: DatastoreMetricsQueryInput;
};


export type QueryDeployArgs = {
  deployId: Scalars['String']['input'];
  serviceId: Scalars['String']['input'];
};


export type QueryDeployHookArgs = {
  serviceId: Scalars['String']['input'];
};


export type QueryDeploysArgs = {
  createdAfter?: InputMaybe<Scalars['String']['input']>;
  createdBefore?: InputMaybe<Scalars['String']['input']>;
  cursor?: InputMaybe<Scalars['String']['input']>;
  finishedAfter?: InputMaybe<Scalars['String']['input']>;
  finishedBefore?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  serviceId: Scalars['String']['input'];
  status?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  updatedAfter?: InputMaybe<Scalars['String']['input']>;
  updatedBefore?: InputMaybe<Scalars['String']['input']>;
};


export type QueryEnvGroupArgs = {
  id: Scalars['String']['input'];
};


export type QueryEnvGroupSecretFileArgs = {
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
};


export type QueryEnvGroupVarArgs = {
  id: Scalars['String']['input'];
  key: Scalars['String']['input'];
};


export type QueryEnvGroupsArgs = {
  cursor?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type QueryEnvVarsArgs = {
  cursor?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  serviceId: Scalars['String']['input'];
};


export type QueryEnvironmentArgs = {
  id: Scalars['String']['input'];
};


export type QueryEnvironmentsArgs = {
  cursor?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  projectId: Scalars['String']['input'];
};


export type QueryGitConnectionArgs = {
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type QueryJobArgs = {
  jobId: Scalars['String']['input'];
  serviceId: Scalars['String']['input'];
};


export type QueryJobsArgs = {
  cursor?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  serviceId: Scalars['String']['input'];
};


export type QueryKeyValueArgs = {
  id: Scalars['String']['input'];
};


export type QueryKeyValueConnectionInfoArgs = {
  id: Scalars['String']['input'];
};


export type QueryKeyValueIpAllowListArgs = {
  id: Scalars['String']['input'];
};


export type QueryKeyValuesArgs = {
  cursor?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type QueryLogLabelValuesArgs = {
  direction?: InputMaybe<Scalars['String']['input']>;
  endTime?: InputMaybe<Scalars['String']['input']>;
  host?: InputMaybe<Array<Scalars['String']['input']>>;
  instance?: InputMaybe<Array<Scalars['String']['input']>>;
  label: Scalars['String']['input'];
  level?: InputMaybe<Array<Scalars['String']['input']>>;
  method?: InputMaybe<Array<Scalars['String']['input']>>;
  path?: InputMaybe<Array<Scalars['String']['input']>>;
  resource: Scalars['String']['input'];
  startTime?: InputMaybe<Scalars['String']['input']>;
  statusCode?: InputMaybe<Array<Scalars['String']['input']>>;
  text?: InputMaybe<Scalars['String']['input']>;
  type?: InputMaybe<Scalars['String']['input']>;
};


export type QueryLogsArgs = {
  direction?: InputMaybe<Scalars['String']['input']>;
  endTime?: InputMaybe<Scalars['String']['input']>;
  host?: InputMaybe<Array<Scalars['String']['input']>>;
  instance?: InputMaybe<Array<Scalars['String']['input']>>;
  level?: InputMaybe<Array<Scalars['String']['input']>>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  method?: InputMaybe<Array<Scalars['String']['input']>>;
  path?: InputMaybe<Array<Scalars['String']['input']>>;
  resource: Scalars['String']['input'];
  startTime?: InputMaybe<Scalars['String']['input']>;
  statusCode?: InputMaybe<Array<Scalars['String']['input']>>;
  text?: InputMaybe<Scalars['String']['input']>;
  type?: InputMaybe<Scalars['String']['input']>;
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
  resourceId: Scalars['String']['input'];
};


export type QueryProjectArgs = {
  id: Scalars['String']['input'];
};


export type QueryProjectsArgs = {
  cursor?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  ownerId: Scalars['String']['input'];
};


export type QueryRegistryCredentialArgs = {
  id: Scalars['String']['input'];
};


export type QueryRegistryCredentialsArgs = {
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type QueryReposArgs = {
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type QuerySecretFilesArgs = {
  cursor?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  serviceId: Scalars['String']['input'];
};


export type QueryServerArgs = {
  id: Scalars['String']['input'];
};


export type QueryServiceArgs = {
  id: Scalars['String']['input'];
};


export type QueryServiceEventsArgs = {
  cursor?: InputMaybe<Scalars['String']['input']>;
  endTime?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  serviceId: Scalars['String']['input'];
  startTime?: InputMaybe<Scalars['String']['input']>;
  type?: InputMaybe<Scalars['String']['input']>;
};


export type QueryServiceNameAvailableArgs = {
  name: Scalars['String']['input'];
};


export type QueryServicesArgs = {
  cursor?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type QueryUsageArgs = {
  ownerId?: InputMaybe<Scalars['String']['input']>;
  period?: InputMaybe<Scalars['String']['input']>;
};


export type QueryValidateBlueprintArgs = {
  bexYaml: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type QueryWebhookDeliveriesArgs = {
  cursor?: InputMaybe<Scalars['String']['input']>;
  endpointId: Scalars['String']['input'];
  limit?: InputMaybe<Scalars['Int']['input']>;
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type QueryWebhookEndpointArgs = {
  id: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type QueryWebhookEndpointsArgs = {
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type QueryWorkspaceInvitesArgs = {
  workspaceId: Scalars['String']['input'];
};


export type QueryWorkspaceLimitsArgs = {
  ownerId: Scalars['String']['input'];
};


export type QueryWorkspaceMembersArgs = {
  workspaceId: Scalars['String']['input'];
};


export type QueryWorkspaceSeatUsageArgs = {
  workspaceId: Scalars['String']['input'];
};

export type ReadReplicaConnectionInfo = {
  __typename: 'ReadReplicaConnectionInfo';
  externalHost: Maybe<Scalars['String']['output']>;
  internalHost: Maybe<Scalars['String']['output']>;
};

export type ReadReplicaView = {
  __typename: 'ReadReplicaView';
  connectionInfo: Maybe<ReadReplicaConnectionInfo>;
  name: Maybe<Scalars['String']['output']>;
};

export type RegistryCredential = {
  __typename: 'RegistryCredential';
  createdAt: Maybe<Scalars['String']['output']>;
  expiresAt: Maybe<Scalars['String']['output']>;
  host: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
  ownerId: Maybe<Scalars['String']['output']>;
  status: Maybe<Scalars['String']['output']>;
  updatedAt: Maybe<Scalars['String']['output']>;
  username: Maybe<Scalars['String']['output']>;
};

export type ReplicaConnectionStrings = {
  __typename: 'ReplicaConnectionStrings';
  externalConnectionString: Maybe<Scalars['String']['output']>;
  internalConnectionString: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
};

export type Repo = {
  __typename: 'Repo';
  cloneUrl: Maybe<Scalars['String']['output']>;
  defaultBranch: Maybe<Scalars['String']['output']>;
  fullName: Maybe<Scalars['String']['output']>;
  htmlUrl: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['Float']['output']>;
  private: Maybe<Scalars['Boolean']['output']>;
};

export type ResourceCap = {
  __typename: 'ResourceCap';
  limit: Maybe<Scalars['Int']['output']>;
  used: Maybe<Scalars['Int']['output']>;
};

export type ResourceLimits = {
  __typename: 'ResourceLimits';
  keyValues: Maybe<ResourceCap>;
  postgres: Maybe<ResourceCap>;
  services: Maybe<ResourceCap>;
};

export type SshKey = {
  __typename: 'SSHKey';
  createdAt: Scalars['String']['output'];
  fingerprint: Scalars['String']['output'];
  id: Scalars['String']['output'];
  name: Scalars['String']['output'];
  publicKey: Scalars['String']['output'];
};

export type SecretFile = {
  __typename: 'SecretFile';
  content: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
};

export type SecretFileInput = {
  content: Scalars['String']['input'];
  name: Scalars['String']['input'];
};

export type SecretFileListValue = {
  __typename: 'SecretFileListValue';
  id: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
};

export type SecretFileWithCursor = {
  __typename: 'SecretFileWithCursor';
  cursor: Maybe<Scalars['String']['output']>;
  secretFile: Maybe<SecretFileListValue>;
};

export type Service = {
  __typename: 'Service';
  autoDeploy: Maybe<Scalars['Boolean']['output']>;
  autoscaling: Maybe<Autoscaling>;
  branch: Maybe<Scalars['String']['output']>;
  buildCommand: Maybe<Scalars['String']['output']>;
  buildFilter: Maybe<BuildFilter>;
  builder: Maybe<Scalars['String']['output']>;
  command: Maybe<Scalars['String']['output']>;
  createdAt: Maybe<Scalars['String']['output']>;
  dashboardUrl: Maybe<Scalars['String']['output']>;
  displayName: Maybe<Scalars['String']['output']>;
  dockerfilePath: Maybe<Scalars['String']['output']>;
  envVar: Maybe<EnvVar>;
  envVarKeys: Maybe<Array<Maybe<EnvVar>>>;
  environmentId: Maybe<Scalars['String']['output']>;
  headers: Maybe<Array<Maybe<StaticHeader>>>;
  healthCheckPath: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  idleTTLSeconds: Maybe<Scalars['Int']['output']>;
  ipAllowList: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  lastSuccessfulRunAt: Maybe<Scalars['String']['output']>;
  latestDeployId: Maybe<Scalars['String']['output']>;
  maintenanceMode: MaintenanceMode;
  maxShutdownDelaySeconds: Maybe<Scalars['Int']['output']>;
  name: Maybe<Scalars['String']['output']>;
  notificationsToSend: Maybe<Scalars['String']['output']>;
  notifyOnFail: Maybe<Scalars['String']['output']>;
  ownerId: Maybe<Scalars['String']['output']>;
  phase: Maybe<Scalars['String']['output']>;
  plan: Maybe<Scalars['String']['output']>;
  preDeployCommand: Maybe<Scalars['String']['output']>;
  projectId: Maybe<Scalars['String']['output']>;
  publishPath: Maybe<Scalars['String']['output']>;
  registryCredentialId: Maybe<Scalars['String']['output']>;
  renderSubdomainPolicy: Maybe<Scalars['String']['output']>;
  replicas: Maybe<Scalars['Int']['output']>;
  repo: Maybe<Scalars['String']['output']>;
  revision: Maybe<Scalars['String']['output']>;
  rootDir: Maybe<Scalars['String']['output']>;
  routes: Maybe<Array<Maybe<StaticRoute>>>;
  runs: Maybe<Array<Maybe<CronRun>>>;
  runtime: Maybe<Scalars['String']['output']>;
  schedule: Maybe<Scalars['String']['output']>;
  secretFile: Maybe<SecretFile>;
  secretFileNames: Maybe<Array<Maybe<SecretFile>>>;
  slug: Maybe<Scalars['String']['output']>;
  sshAddress: Maybe<Scalars['String']['output']>;
  startCommand: Maybe<Scalars['String']['output']>;
  suspended: Maybe<Scalars['String']['output']>;
  suspenders: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  type: Maybe<Scalars['String']['output']>;
  updatedAt: Maybe<Scalars['String']['output']>;
  url: Maybe<Scalars['String']['output']>;
};


export type ServiceEnvVarArgs = {
  key: Scalars['String']['input'];
};


export type ServiceSecretFileArgs = {
  name: Scalars['String']['input'];
};

export type ServiceEvent = {
  __typename: 'ServiceEvent';
  cursor: Maybe<Scalars['String']['output']>;
  details: Maybe<ServiceEventDetails>;
  id: Maybe<Scalars['String']['output']>;
  serviceId: Maybe<Scalars['String']['output']>;
  timestamp: Maybe<Scalars['String']['output']>;
  type: Maybe<Scalars['String']['output']>;
};

export type ServiceEventDetails = {
  __typename: 'ServiceEventDetails';
  actor: Maybe<Scalars['String']['output']>;
  autoscalingMaxFrom: Maybe<Scalars['Int']['output']>;
  autoscalingMaxTo: Maybe<Scalars['Int']['output']>;
  autoscalingMinFrom: Maybe<Scalars['Int']['output']>;
  autoscalingMinTo: Maybe<Scalars['Int']['output']>;
  deployId: Maybe<Scalars['String']['output']>;
  deployStatus: Maybe<Scalars['String']['output']>;
  instanceCountFrom: Maybe<Scalars['Int']['output']>;
  instanceCountTo: Maybe<Scalars['Int']['output']>;
  planFrom: Maybe<Scalars['String']['output']>;
  planTo: Maybe<Scalars['String']['output']>;
  preDeployStatus: Maybe<Scalars['String']['output']>;
  trigger: Maybe<DeployTrigger>;
  triggeredByUser: Maybe<Scalars['String']['output']>;
};

export type ServiceUsage = {
  __typename: 'ServiceUsage';
  resourceKind: Maybe<Scalars['String']['output']>;
  rows: Maybe<Array<Maybe<UsageRow>>>;
  serviceId: Maybe<Scalars['String']['output']>;
};

export type StaticHeader = {
  __typename: 'StaticHeader';
  name: Maybe<Scalars['String']['output']>;
  path: Maybe<Scalars['String']['output']>;
  value: Maybe<Scalars['String']['output']>;
};

export type StaticHeaderInput = {
  name: Scalars['String']['input'];
  path: Scalars['String']['input'];
  value: Scalars['String']['input'];
};

export type StaticRoute = {
  __typename: 'StaticRoute';
  destination: Maybe<Scalars['String']['output']>;
  source: Maybe<Scalars['String']['output']>;
  type: Maybe<Scalars['String']['output']>;
};

export type StaticRouteInput = {
  destination: Scalars['String']['input'];
  source: Scalars['String']['input'];
  type: Scalars['String']['input'];
};

export type SyncBlueprintResult = {
  __typename: 'SyncBlueprintResult';
  blueprint: Maybe<Blueprint>;
  databases: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  services: Maybe<Array<Maybe<Service>>>;
};

export type TableSizeInfo = {
  __typename: 'TableSizeInfo';
  name: Maybe<Scalars['String']['output']>;
  schema: Maybe<Scalars['String']['output']>;
  sizeBytes: Maybe<Scalars['Int']['output']>;
  sizePretty: Maybe<Scalars['String']['output']>;
};

export type UsageRow = {
  __typename: 'UsageRow';
  kind: Maybe<Scalars['String']['output']>;
  tier: Maybe<Scalars['String']['output']>;
  total: Maybe<Scalars['Float']['output']>;
};

export type UsageSummary = {
  __typename: 'UsageSummary';
  estimatedCost: Maybe<EstimatedCost>;
  period: Maybe<Scalars['String']['output']>;
  services: Maybe<Array<Maybe<ServiceUsage>>>;
  workspaceId: Maybe<Scalars['String']['output']>;
};

export type WebhookDelivery = {
  __typename: 'WebhookDelivery';
  attemptCount: Maybe<Scalars['Int']['output']>;
  createdAt: Maybe<Scalars['String']['output']>;
  cursor: Maybe<Scalars['String']['output']>;
  deliveredAt: Maybe<Scalars['String']['output']>;
  eventId: Maybe<Scalars['String']['output']>;
  eventType: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  lastAttemptedAt: Maybe<Scalars['String']['output']>;
  lastError: Maybe<Scalars['String']['output']>;
  lastStatusCode: Maybe<Scalars['Int']['output']>;
  nextAttemptAt: Maybe<Scalars['String']['output']>;
  serviceId: Maybe<Scalars['String']['output']>;
  status: Maybe<Scalars['String']['output']>;
};

export type WebhookEndpoint = {
  __typename: 'WebhookEndpoint';
  createdAt: Maybe<Scalars['String']['output']>;
  createdBy: Maybe<Scalars['String']['output']>;
  disabledReason: Maybe<Scalars['String']['output']>;
  enabled: Maybe<Scalars['Boolean']['output']>;
  eventTypes: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  id: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
  ownerId: Maybe<Scalars['String']['output']>;
  secret: Maybe<Scalars['String']['output']>;
  updatedAt: Maybe<Scalars['String']['output']>;
  url: Maybe<Scalars['String']['output']>;
};

export type Workspace = {
  __typename: 'Workspace';
  createdAt: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
  plan: Maybe<Scalars['String']['output']>;
  role: Maybe<Scalars['String']['output']>;
};

export type WorkspaceInvite = {
  __typename: 'WorkspaceInvite';
  createdAt: Maybe<Scalars['String']['output']>;
  email: Maybe<Scalars['String']['output']>;
  expiresAt: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  role: Maybe<Scalars['String']['output']>;
};

export type WorkspaceMember = {
  __typename: 'WorkspaceMember';
  createdAt: Maybe<Scalars['String']['output']>;
  email: Maybe<Scalars['String']['output']>;
  mfaEnabled: Maybe<Scalars['Boolean']['output']>;
  role: Maybe<Scalars['String']['output']>;
  subject: Maybe<Scalars['String']['output']>;
  userId: Maybe<Scalars['String']['output']>;
};

export type WorkspaceSeatUsage = {
  __typename: 'WorkspaceSeatUsage';
  limit: Maybe<Scalars['Int']['output']>;
  used: Maybe<Scalars['Int']['output']>;
};

export type ApiKeysQueryVariables = Exact<{
  ownerId?: InputMaybe<Scalars['String']['input']>;
}>;


export type ApiKeysQuery = { apiKeys: Array<{ __typename: 'ApiKey', id: string | null, name: string | null, createdAt: string | null, createdBy: string | null, lastUsedAt: string | null } | null> | null };

export type CreateApiKeyMutationVariables = Exact<{
  name: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
}>;


export type CreateApiKeyMutation = { createApiKey: { __typename: 'ApiKey', id: string | null, name: string | null, secret: string | null, createdAt: string | null } | null };

export type RevokeApiKeyMutationVariables = Exact<{
  id: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
}>;


export type RevokeApiKeyMutation = { revokeApiKey: boolean | null };

export type AuditLogsQueryVariables = Exact<{
  ownerId: Scalars['String']['input'];
  startTime?: InputMaybe<Scalars['String']['input']>;
  endTime?: InputMaybe<Scalars['String']['input']>;
  cursor?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
}>;


export type AuditLogsQuery = { auditLogs: Array<{ __typename: 'AuditLog', id: string | null, timestamp: string | null, actor: string | null, actorMethod: string | null, action: string | null, status: string | null, resource: string | null, targetName: string | null } | null> | null };

export type DatabasesQueryVariables = Exact<{
  ownerId?: InputMaybe<Scalars['String']['input']>;
}>;


export type DatabasesQuery = { databases: Array<{ __typename: 'Database', id: string | null, name: string | null, plan: string | null, version: string | null, status: string | null, diskSizeGB: number | null, diskAutoscalingEnabled: boolean | null, suspended: string | null, createdAt: string | null, public: boolean | null } | null> | null };

export type DatabaseQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DatabaseQuery = { database: { __typename: 'Database', id: string | null, name: string | null, plan: string | null, version: string | null, status: string | null, databaseName: string | null, databaseUser: string | null, diskSizeGB: number | null, diskAutoscalingEnabled: boolean | null, highAvailabilityEnabled: boolean | null, suspended: string | null, createdAt: string | null, externalHost: string | null, public: boolean | null, poolerEnabled: boolean | null, backupsEnabled: boolean | null, ipAllowList: Array<string | null> | null, readReplicas: Array<{ __typename: 'ReadReplicaView', name: string | null, connectionInfo: { __typename: 'ReadReplicaConnectionInfo', internalHost: string | null, externalHost: string | null } | null } | null> | null } | null };

export type DatabaseConnectionInfoQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DatabaseConnectionInfoQuery = { databaseConnectionInfo: { __typename: 'PostgresConnectionInfo', password: string | null, internalConnectionString: string | null, externalConnectionString: string | null, internalConnectionPoolString: string | null, externalConnectionPoolString: string | null, psqlCommand: string | null, readReplicaConnectionStrings: Array<{ __typename: 'ReplicaConnectionStrings', name: string | null, internalConnectionString: string | null, externalConnectionString: string | null } | null> | null } | null };

export type DatabaseRecoveryInfoQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DatabaseRecoveryInfoQuery = { databaseRecoveryInfo: { __typename: 'DatabaseRecoveryInfo', enabled: boolean | null, earliestRecoveryTime: string | null, latestRecoveryTime: string | null, backups: Array<{ __typename: 'DatabaseBackup', id: string | null, status: string | null, createdAt: string | null } | null> | null } | null };

export type DatabaseExportsQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DatabaseExportsQuery = { databaseExports: Array<{ __typename: 'DatabaseExport', id: string | null, status: string | null, createdAt: string | null, url: string | null, urlExpiresAt: string | null, expiresAt: string | null, filename: string | null, failureReason: string | null } | null> | null };

export type DatabaseUsersQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DatabaseUsersQuery = { databaseUsers: Array<{ __typename: 'DatabaseUser', name: string | null } | null> | null };

export type DatabaseIpAllowListQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DatabaseIpAllowListQuery = { database: { __typename: 'Database', id: string | null, ipAllowListEntries: Array<{ __typename: 'IPAllowListEntry', cidrBlock: string, description: string | null } | null> | null } | null };

export type FailoverDatabaseMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type FailoverDatabaseMutation = { failoverDatabase: boolean | null };

export type SuspendDatabaseMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type SuspendDatabaseMutation = { suspendDatabase: { __typename: 'Database', id: string | null, suspended: string | null, status: string | null } | null };

export type ResumeDatabaseMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type ResumeDatabaseMutation = { resumeDatabase: { __typename: 'Database', id: string | null, suspended: string | null, status: string | null } | null };

export type RestartDatabaseMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type RestartDatabaseMutation = { restartDatabase: { __typename: 'Database', id: string | null, suspended: string | null, status: string | null } | null };

export type RecoverDatabaseMutationVariables = Exact<{
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
  targetTime?: InputMaybe<Scalars['String']['input']>;
  plan?: InputMaybe<Scalars['String']['input']>;
  version?: InputMaybe<Scalars['String']['input']>;
}>;


export type RecoverDatabaseMutation = { recoverDatabase: { __typename: 'Database', id: string | null, name: string | null, plan: string | null, status: string | null } | null };

export type CreateDatabaseExportMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type CreateDatabaseExportMutation = { createDatabaseExport: { __typename: 'DatabaseExport', id: string | null, status: string | null, createdAt: string | null, url: string | null, urlExpiresAt: string | null, expiresAt: string | null, filename: string | null, failureReason: string | null } | null };

export type SetDatabaseIpAllowListMutationVariables = Exact<{
  id: Scalars['String']['input'];
  entries?: InputMaybe<Array<IpAllowListEntryInput> | IpAllowListEntryInput>;
}>;


export type SetDatabaseIpAllowListMutation = { setDatabaseIpAllowList: { __typename: 'Database', id: string | null, ipAllowListEntries: Array<{ __typename: 'IPAllowListEntry', cidrBlock: string, description: string | null } | null> | null } | null };

export type CreateDatabaseUserMutationVariables = Exact<{
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
}>;


export type CreateDatabaseUserMutation = { createDatabaseUser: { __typename: 'DatabaseUserWithPassword', name: string | null, password: string | null } | null };

export type DeleteDatabaseUserMutationVariables = Exact<{
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
}>;


export type DeleteDatabaseUserMutation = { deleteDatabaseUser: boolean | null };

export type DatabaseInstanceTypesQueryVariables = Exact<{ [key: string]: never; }>;


export type DatabaseInstanceTypesQuery = { databaseInstanceTypes: Array<{ __typename: 'DatabaseInstanceType', id: string | null, name: string | null, cpu: string | null, memory: string | null, storageGB: number | null } | null> | null };

export type CreateDatabaseMutationVariables = Exact<{
  name: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
  environmentId?: InputMaybe<Scalars['String']['input']>;
  plan?: InputMaybe<Scalars['String']['input']>;
  version?: InputMaybe<Scalars['String']['input']>;
  diskSizeGB?: InputMaybe<Scalars['Int']['input']>;
  public?: InputMaybe<Scalars['Boolean']['input']>;
}>;


export type CreateDatabaseMutation = { createDatabase: { __typename: 'Database', id: string | null, name: string | null, plan: string | null, status: string | null, projectId: string | null, environmentId: string | null } | null };

export type UpdateDatabasePlanMutationVariables = Exact<{
  id: Scalars['String']['input'];
  plan: Scalars['String']['input'];
}>;


export type UpdateDatabasePlanMutation = { updateDatabasePlan: { __typename: 'Database', id: string | null, name: string | null, plan: string | null, status: string | null } | null };

export type UpdateDatabaseVersionMutationVariables = Exact<{
  id: Scalars['String']['input'];
  version: Scalars['String']['input'];
}>;


export type UpdateDatabaseVersionMutation = { updateDatabaseVersion: { __typename: 'Database', id: string | null, version: string | null, status: string | null } | null };

export type UpdateDatabaseDiskAutoscalingMutationVariables = Exact<{
  id: Scalars['String']['input'];
  enabled: Scalars['Boolean']['input'];
}>;


export type UpdateDatabaseDiskAutoscalingMutation = { updateDatabaseDiskAutoscaling: { __typename: 'Database', id: string | null, diskSizeGB: number | null, diskAutoscalingEnabled: boolean | null } | null };

export type RenameDatabaseMutationVariables = Exact<{
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
}>;


export type RenameDatabaseMutation = { renameDatabase: { __typename: 'Database', id: string | null, name: string | null, databaseName: string | null, databaseUser: string | null, externalHost: string | null } | null };

export type DeleteDatabaseMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DeleteDatabaseMutation = { deleteDatabase: boolean | null };

export type DatabaseProcessesQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DatabaseProcessesQuery = { databaseProcesses: Array<{ __typename: 'DatabaseProcess', pid: number | null, userName: string | null, applicationName: string | null, state: string | null, query: string | null, waitEventType: string | null, waitEvent: string | null, durationSeconds: number | null } | null> | null };

export type DatabaseTopQueriesQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DatabaseTopQueriesQuery = { databaseTopQueries: Array<{ __typename: 'DatabaseTopQuery', query: string | null, calls: number | null, totalTimeMs: number | null, meanTimeMs: number | null, rows: number | null, sharedHitBlks: number | null, sharedReadBlks: number | null } | null> | null };

export type DatabaseSizesQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DatabaseSizesQuery = { databaseSizes: { __typename: 'DatabaseSizes', database: { __typename: 'DatabaseSizeInfo', name: string | null, sizeBytes: number | null, sizePretty: string | null } | null, tables: Array<{ __typename: 'TableSizeInfo', schema: string | null, name: string | null, sizeBytes: number | null, sizePretty: string | null } | null> | null } | null };

export type DatabaseTableScansQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DatabaseTableScansQuery = { databaseTableScans: Array<{ __typename: 'DatabaseTableScan', schema: string | null, name: string | null, seqScans: number | null, seqScanRows: number | null, indexScans: number | null, indexScanRows: number | null, liveRows: number | null, deadRows: number | null } | null> | null };

export type DatabaseParameterOverridesQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DatabaseParameterOverridesQuery = { databaseParameterOverrides: Array<{ __typename: 'DatabaseParameterOverride', name: string | null, setting: string | null, unit: string | null, source: string | null, description: string | null } | null> | null };

export type SetDatabaseParameterOverridesMutationVariables = Exact<{
  id: Scalars['String']['input'];
  parameters?: InputMaybe<Array<ParameterInput> | ParameterInput>;
}>;


export type SetDatabaseParameterOverridesMutation = { setDatabaseParameterOverrides: { __typename: 'Database', id: string | null, name: string | null } | null };

export type ExecuteDatabaseQueryMutationVariables = Exact<{
  id: Scalars['String']['input'];
  sql: Scalars['String']['input'];
  allowWrites?: InputMaybe<Scalars['Boolean']['input']>;
}>;


export type ExecuteDatabaseQueryMutation = { executeDatabaseQuery: { __typename: 'DatabaseQueryResult', columns: Array<string | null> | null, rowCount: number | null, truncated: boolean | null, rows: Array<{ __typename: 'DatabaseQueryRow', values: Array<string | null> | null } | null> | null } | null };

export type DeployQueryVariables = Exact<{
  serviceId: Scalars['String']['input'];
  deployId: Scalars['String']['input'];
}>;


export type DeployQuery = { deploy: { __typename: 'Deploy', id: string | null, status: string | null, trigger: string | null, image: string | null, rollbackOf: string | null, commitId: string | null, commitMessage: string | null, commitCreatedAt: string | null, createdAt: string | null, updatedAt: string | null, startedAt: string | null, finishedAt: string | null, preDeployStatus: string | null } | null };

export type DeployTimelineEventsQueryVariables = Exact<{
  serviceId: Scalars['String']['input'];
  startTime?: InputMaybe<Scalars['String']['input']>;
  endTime?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
}>;


export type DeployTimelineEventsQuery = { serviceEvents: Array<{ __typename: 'ServiceEvent', id: string | null, type: string | null, timestamp: string | null, details: { __typename: 'ServiceEventDetails', deployId: string | null, deployStatus: string | null } | null } | null> | null };

export type DeploysQueryVariables = Exact<{
  serviceId: Scalars['String']['input'];
  status?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>> | InputMaybe<Scalars['String']['input']>>;
  createdBefore?: InputMaybe<Scalars['String']['input']>;
  createdAfter?: InputMaybe<Scalars['String']['input']>;
  updatedBefore?: InputMaybe<Scalars['String']['input']>;
  updatedAfter?: InputMaybe<Scalars['String']['input']>;
  finishedBefore?: InputMaybe<Scalars['String']['input']>;
  finishedAfter?: InputMaybe<Scalars['String']['input']>;
  cursor?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
}>;


export type DeploysQuery = { deploys: Array<{ __typename: 'Deploy', id: string | null, status: string | null, trigger: string | null, image: string | null, rollbackOf: string | null, commitId: string | null, commitMessage: string | null, commitCreatedAt: string | null, createdAt: string | null, updatedAt: string | null, startedAt: string | null, finishedAt: string | null, preDeployStatus: string | null } | null> | null };

export type EnvGroupsQueryVariables = Exact<{
  ownerId?: InputMaybe<Scalars['String']['input']>;
}>;


export type EnvGroupsQuery = { envGroups: Array<{ __typename: 'EnvGroup', id: string | null, name: string | null, ownerId: string | null, createdAt: string | null, updatedAt: string | null, serviceLinks: Array<string | null> | null, envVars: Array<{ __typename: 'EnvGroupVar', key: string | null } | null> | null, secretFiles: Array<{ __typename: 'EnvGroupSecretFile', name: string | null } | null> | null } | null> | null };

export type EnvGroupQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type EnvGroupQuery = { envGroup: { __typename: 'EnvGroup', id: string | null, name: string | null, ownerId: string | null, createdAt: string | null, updatedAt: string | null, serviceLinks: Array<string | null> | null, envVars: Array<{ __typename: 'EnvGroupVar', key: string | null } | null> | null, secretFiles: Array<{ __typename: 'EnvGroupSecretFile', name: string | null } | null> | null } | null };

export type EnvGroupVarValueQueryVariables = Exact<{
  id: Scalars['String']['input'];
  key: Scalars['String']['input'];
}>;


export type EnvGroupVarValueQuery = { envGroupVar: { __typename: 'EnvGroupVar', key: string | null, value: string | null } | null };

export type EnvGroupSecretFileContentQueryVariables = Exact<{
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
}>;


export type EnvGroupSecretFileContentQuery = { envGroupSecretFile: { __typename: 'EnvGroupSecretFile', name: string | null, content: string | null } | null };

export type CreateEnvGroupMutationVariables = Exact<{
  name: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
  envVars?: InputMaybe<Array<EnvGroupVarInput> | EnvGroupVarInput>;
  secretFiles?: InputMaybe<Array<EnvGroupSecretFileInput> | EnvGroupSecretFileInput>;
  serviceIds?: InputMaybe<Array<Scalars['String']['input']> | Scalars['String']['input']>;
}>;


export type CreateEnvGroupMutation = { createEnvGroup: { __typename: 'EnvGroup', id: string | null, name: string | null } | null };

export type RenameEnvGroupMutationVariables = Exact<{
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
}>;


export type RenameEnvGroupMutation = { renameEnvGroup: { __typename: 'EnvGroup', id: string | null, name: string | null } | null };

export type DeleteEnvGroupMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DeleteEnvGroupMutation = { deleteEnvGroup: boolean | null };

export type SetEnvGroupVarsMutationVariables = Exact<{
  id: Scalars['String']['input'];
  envVars: Array<EnvGroupVarInput> | EnvGroupVarInput;
}>;


export type SetEnvGroupVarsMutation = { setEnvGroupVars: boolean | null };

export type SetEnvGroupVarMutationVariables = Exact<{
  id: Scalars['String']['input'];
  key: Scalars['String']['input'];
  value?: InputMaybe<Scalars['String']['input']>;
}>;


export type SetEnvGroupVarMutation = { setEnvGroupVar: boolean | null };

export type DeleteEnvGroupVarMutationVariables = Exact<{
  id: Scalars['String']['input'];
  key: Scalars['String']['input'];
}>;


export type DeleteEnvGroupVarMutation = { deleteEnvGroupVar: boolean | null };

export type SetEnvGroupSecretFileMutationVariables = Exact<{
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
  content?: InputMaybe<Scalars['String']['input']>;
}>;


export type SetEnvGroupSecretFileMutation = { setEnvGroupSecretFile: boolean | null };

export type DeleteEnvGroupSecretFileMutationVariables = Exact<{
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
}>;


export type DeleteEnvGroupSecretFileMutation = { deleteEnvGroupSecretFile: boolean | null };

export type LinkEnvGroupMutationVariables = Exact<{
  id: Scalars['String']['input'];
  serviceId: Scalars['String']['input'];
}>;


export type LinkEnvGroupMutation = { linkEnvGroup: boolean | null };

export type UnlinkEnvGroupMutationVariables = Exact<{
  id: Scalars['String']['input'];
  serviceId: Scalars['String']['input'];
}>;


export type UnlinkEnvGroupMutation = { unlinkEnvGroup: boolean | null };

export type EnvironmentFieldsFragment = { __typename: 'Environment', id: string | null, projectId: string | null, name: string | null, ownerId: string | null, createdAt: string | null, serviceIds: Array<string | null> | null, databaseIds: Array<string | null> | null, keyValueIds: Array<string | null> | null, envGroupIds: Array<string | null> | null, protectedStatus: string | null, networkIsolationEnabled: boolean | null, ipAllowList: Array<string | null> | null, ipAllowListEntries: Array<{ __typename: 'IPAllowListEntry', cidrBlock: string, description: string | null } | null> | null };

export type EnvironmentsQueryVariables = Exact<{
  projectId: Scalars['String']['input'];
}>;


export type EnvironmentsQuery = { environments: Array<{ __typename: 'Environment', id: string | null, projectId: string | null, name: string | null, ownerId: string | null, createdAt: string | null, serviceIds: Array<string | null> | null, databaseIds: Array<string | null> | null, keyValueIds: Array<string | null> | null, envGroupIds: Array<string | null> | null, protectedStatus: string | null, networkIsolationEnabled: boolean | null, ipAllowList: Array<string | null> | null, ipAllowListEntries: Array<{ __typename: 'IPAllowListEntry', cidrBlock: string, description: string | null } | null> | null } | null> | null };

export type CreateEnvironmentMutationVariables = Exact<{
  name: Scalars['String']['input'];
  projectId: Scalars['String']['input'];
}>;


export type CreateEnvironmentMutation = { createEnvironment: { __typename: 'Environment', id: string | null, projectId: string | null, name: string | null, ownerId: string | null, createdAt: string | null, serviceIds: Array<string | null> | null, databaseIds: Array<string | null> | null, keyValueIds: Array<string | null> | null, envGroupIds: Array<string | null> | null, protectedStatus: string | null, networkIsolationEnabled: boolean | null, ipAllowList: Array<string | null> | null, ipAllowListEntries: Array<{ __typename: 'IPAllowListEntry', cidrBlock: string, description: string | null } | null> | null } | null };

export type RenameEnvironmentMutationVariables = Exact<{
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
}>;


export type RenameEnvironmentMutation = { renameEnvironment: { __typename: 'Environment', id: string | null, projectId: string | null, name: string | null, ownerId: string | null, createdAt: string | null, serviceIds: Array<string | null> | null, databaseIds: Array<string | null> | null, keyValueIds: Array<string | null> | null, envGroupIds: Array<string | null> | null, protectedStatus: string | null, networkIsolationEnabled: boolean | null, ipAllowList: Array<string | null> | null, ipAllowListEntries: Array<{ __typename: 'IPAllowListEntry', cidrBlock: string, description: string | null } | null> | null } | null };

export type DeleteEnvironmentMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DeleteEnvironmentMutation = { deleteEnvironment: string | null };

export type SetEnvironmentServicesMutationVariables = Exact<{
  id: Scalars['String']['input'];
  serviceIds: Array<Scalars['String']['input']> | Scalars['String']['input'];
}>;


export type SetEnvironmentServicesMutation = { setEnvironmentServices: { __typename: 'Environment', id: string | null, projectId: string | null, name: string | null, ownerId: string | null, createdAt: string | null, serviceIds: Array<string | null> | null, databaseIds: Array<string | null> | null, keyValueIds: Array<string | null> | null, envGroupIds: Array<string | null> | null, protectedStatus: string | null, networkIsolationEnabled: boolean | null, ipAllowList: Array<string | null> | null, ipAllowListEntries: Array<{ __typename: 'IPAllowListEntry', cidrBlock: string, description: string | null } | null> | null } | null };

export type SetEnvironmentDatabasesMutationVariables = Exact<{
  id: Scalars['String']['input'];
  databaseIds: Array<Scalars['String']['input']> | Scalars['String']['input'];
}>;


export type SetEnvironmentDatabasesMutation = { setEnvironmentDatabases: { __typename: 'Environment', id: string | null, projectId: string | null, name: string | null, ownerId: string | null, createdAt: string | null, serviceIds: Array<string | null> | null, databaseIds: Array<string | null> | null, keyValueIds: Array<string | null> | null, envGroupIds: Array<string | null> | null, protectedStatus: string | null, networkIsolationEnabled: boolean | null, ipAllowList: Array<string | null> | null, ipAllowListEntries: Array<{ __typename: 'IPAllowListEntry', cidrBlock: string, description: string | null } | null> | null } | null };

export type SetEnvironmentKeyValuesMutationVariables = Exact<{
  id: Scalars['String']['input'];
  keyValueIds: Array<Scalars['String']['input']> | Scalars['String']['input'];
}>;


export type SetEnvironmentKeyValuesMutation = { setEnvironmentKeyValues: { __typename: 'Environment', id: string | null, projectId: string | null, name: string | null, ownerId: string | null, createdAt: string | null, serviceIds: Array<string | null> | null, databaseIds: Array<string | null> | null, keyValueIds: Array<string | null> | null, envGroupIds: Array<string | null> | null, protectedStatus: string | null, networkIsolationEnabled: boolean | null, ipAllowList: Array<string | null> | null, ipAllowListEntries: Array<{ __typename: 'IPAllowListEntry', cidrBlock: string, description: string | null } | null> | null } | null };

export type SetEnvironmentEnvGroupsMutationVariables = Exact<{
  id: Scalars['String']['input'];
  envGroupIds: Array<Scalars['String']['input']> | Scalars['String']['input'];
}>;


export type SetEnvironmentEnvGroupsMutation = { setEnvironmentEnvGroups: { __typename: 'Environment', id: string | null, projectId: string | null, name: string | null, ownerId: string | null, createdAt: string | null, serviceIds: Array<string | null> | null, databaseIds: Array<string | null> | null, keyValueIds: Array<string | null> | null, envGroupIds: Array<string | null> | null, protectedStatus: string | null, networkIsolationEnabled: boolean | null, ipAllowList: Array<string | null> | null, ipAllowListEntries: Array<{ __typename: 'IPAllowListEntry', cidrBlock: string, description: string | null } | null> | null } | null };

export type SetEnvironmentAclMutationVariables = Exact<{
  id: Scalars['String']['input'];
  protectedStatus: Scalars['String']['input'];
  networkIsolationEnabled: Scalars['Boolean']['input'];
  ipAllowListEntries?: InputMaybe<Array<IpAllowListEntryInput> | IpAllowListEntryInput>;
}>;


export type SetEnvironmentAclMutation = { setEnvironmentACL: { __typename: 'Environment', id: string | null, projectId: string | null, name: string | null, ownerId: string | null, createdAt: string | null, serviceIds: Array<string | null> | null, databaseIds: Array<string | null> | null, keyValueIds: Array<string | null> | null, envGroupIds: Array<string | null> | null, protectedStatus: string | null, networkIsolationEnabled: boolean | null, ipAllowList: Array<string | null> | null, ipAllowListEntries: Array<{ __typename: 'IPAllowListEntry', cidrBlock: string, description: string | null } | null> | null } | null };

export type ReposQueryVariables = Exact<{ [key: string]: never; }>;


export type ReposQuery = { repos: Array<{ __typename: 'Repo', id: number | null, fullName: string | null, private: boolean | null, defaultBranch: string | null, htmlUrl: string | null, cloneUrl: string | null } | null> | null };

export type GitConnectionQueryVariables = Exact<{ [key: string]: never; }>;


export type GitConnectionQuery = { gitConnection: { __typename: 'GitConnection', connected: boolean | null, accountLogin: string | null, installUrl: string | null } | null };

export type ConnectGitMutationVariables = Exact<{ [key: string]: never; }>;


export type ConnectGitMutation = { connectGit: { __typename: 'GitConnection', connected: boolean | null, installUrl: string | null } | null };

export type DisconnectGitMutationVariables = Exact<{ [key: string]: never; }>;


export type DisconnectGitMutation = { disconnectGit: boolean | null };

export type KeyValuesQueryVariables = Exact<{
  ownerId?: InputMaybe<Scalars['String']['input']>;
}>;


export type KeyValuesQuery = { keyValues: Array<{ __typename: 'KeyValue', id: string | null, name: string | null, plan: string | null, version: string | null, status: string | null, suspended: string | null, createdAt: string | null, externalHost: string | null, public: boolean | null } | null> | null };

export type KeyValueQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type KeyValueQuery = { keyValue: { __typename: 'KeyValue', id: string | null, name: string | null, plan: string | null, version: string | null, status: string | null, suspended: string | null, createdAt: string | null, externalHost: string | null, public: boolean | null } | null };

export type KeyValueConnectionInfoQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type KeyValueConnectionInfoQuery = { keyValueConnectionInfo: { __typename: 'KeyValueConnectionInfo', internalConnectionString: string | null, externalConnectionString: string | null, cliCommand: string | null } | null };

export type KeyValueInstanceTypesQueryVariables = Exact<{ [key: string]: never; }>;


export type KeyValueInstanceTypesQuery = { keyValueInstanceTypes: Array<{ __typename: 'KeyValueInstanceType', id: string | null, name: string | null, cpu: string | null, memory: string | null, storageGB: number | null } | null> | null };

export type KeyValueIpAllowListQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type KeyValueIpAllowListQuery = { keyValue: { __typename: 'KeyValue', id: string | null, ipAllowListEntries: Array<{ __typename: 'IPAllowListEntry', cidrBlock: string, description: string | null } | null> | null } | null };

export type CreateKeyValueMutationVariables = Exact<{
  name: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
  environmentId?: InputMaybe<Scalars['String']['input']>;
  plan?: InputMaybe<Scalars['String']['input']>;
  version?: InputMaybe<Scalars['String']['input']>;
  storageGB?: InputMaybe<Scalars['Int']['input']>;
  public?: InputMaybe<Scalars['Boolean']['input']>;
  maxmemoryPolicy?: InputMaybe<Scalars['String']['input']>;
  persistenceMode?: InputMaybe<Scalars['String']['input']>;
}>;


export type CreateKeyValueMutation = { createKeyValue: { __typename: 'KeyValue', id: string | null, name: string | null, plan: string | null, status: string | null, projectId: string | null, environmentId: string | null } | null };

export type SetKeyValueIpAllowListMutationVariables = Exact<{
  id: Scalars['String']['input'];
  entries?: InputMaybe<Array<IpAllowListEntryInput> | IpAllowListEntryInput>;
}>;


export type SetKeyValueIpAllowListMutation = { setKeyValueIpAllowList: { __typename: 'KeyValue', id: string | null, ipAllowListEntries: Array<{ __typename: 'IPAllowListEntry', cidrBlock: string, description: string | null } | null> | null } | null };

export type UpdateKeyValuePlanMutationVariables = Exact<{
  id: Scalars['String']['input'];
  plan: Scalars['String']['input'];
}>;


export type UpdateKeyValuePlanMutation = { updateKeyValuePlan: { __typename: 'KeyValue', id: string | null, name: string | null, plan: string | null, status: string | null } | null };

export type RenameKeyValueMutationVariables = Exact<{
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
}>;


export type RenameKeyValueMutation = { renameKeyValue: { __typename: 'KeyValue', id: string | null, name: string | null } | null };

export type DeleteKeyValueMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DeleteKeyValueMutation = { deleteKeyValue: boolean | null };

export type SuspendKeyValueMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type SuspendKeyValueMutation = { suspendKeyValue: { __typename: 'KeyValue', id: string | null, suspended: string | null } | null };

export type ResumeKeyValueMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type ResumeKeyValueMutation = { resumeKeyValue: { __typename: 'KeyValue', id: string | null, suspended: string | null } | null };

export type LogsQueryVariables = Exact<{
  resource: Scalars['String']['input'];
  type?: InputMaybe<Scalars['String']['input']>;
  text?: InputMaybe<Scalars['String']['input']>;
  level?: InputMaybe<Array<Scalars['String']['input']> | Scalars['String']['input']>;
  instance?: InputMaybe<Array<Scalars['String']['input']> | Scalars['String']['input']>;
  statusCode?: InputMaybe<Array<Scalars['String']['input']> | Scalars['String']['input']>;
  method?: InputMaybe<Array<Scalars['String']['input']> | Scalars['String']['input']>;
  path?: InputMaybe<Array<Scalars['String']['input']> | Scalars['String']['input']>;
  startTime?: InputMaybe<Scalars['String']['input']>;
  endTime?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
}>;


export type LogsQuery = { logs: Array<{ __typename: 'LogEntry', timestamp: string | null, message: string | null, type: string | null, instance: string | null, level: string | null, method: string | null, statusCode: string | null } | null> | null };

export type LogLabelValuesQueryVariables = Exact<{
  label: Scalars['String']['input'];
  resource: Scalars['String']['input'];
}>;


export type LogLabelValuesQuery = { logLabelValues: Array<string> | null };

export type MetricsQueryVariables = Exact<{
  query: MetricsQueryInput;
}>;


export type MetricsQuery = { metrics: Array<{ __typename: 'MetricSeries', unit: string | null, labels: Array<{ __typename: 'MetricLabel', field: string | null, value: string | null } | null> | null, values: Array<{ __typename: 'MetricValue', time: string | null, value: number | null } | null> | null, parameters: Array<{ __typename: 'MetricSeriesParameter', quantile: number | null } | null> | null } | null> | null };

export type MonthToDateBandwidthQueryVariables = Exact<{
  resourceId: Scalars['String']['input'];
}>;


export type MonthToDateBandwidthQuery = { monthToDateBandwidth: { __typename: 'MonthToDateBandwidth', egressBandwidthMB: number | null, httpEgressBandwidthMB: number | null, natEgressBandwidthMB: number | null, privateLinkEgressBandwidthMB: number | null, websocketEgressBandwidthMB: number | null } | null };

export type MetricsFiltersQueryVariables = Exact<{
  query: MetricsFiltersQueryInput;
}>;


export type MetricsFiltersQuery = { metricsFilters: { __typename: 'MetricsFiltersResult', values: Array<{ __typename: 'MetricsFilterValues', field: string | null, values: Array<string | null> | null } | null> | null } | null };

export type MetricsPathFilterSuggestionsQueryVariables = Exact<{
  query: MetricsPathFilterSuggestionsInput;
}>;


export type MetricsPathFilterSuggestionsQuery = { metricsPathFilterSuggestions: { __typename: 'MetricsPathFilterSuggestions', paths: Array<string | null> | null } | null };

export type DatastoreMetricsQueryVariables = Exact<{
  query: DatastoreMetricsQueryInput;
}>;


export type DatastoreMetricsQuery = { datastoreMetrics: Array<{ __typename: 'MetricSeries', unit: string | null, labels: Array<{ __typename: 'MetricLabel', field: string | null, value: string | null } | null> | null, values: Array<{ __typename: 'MetricValue', time: string | null, value: number | null } | null> | null } | null> | null };

export type NotificationSettingsQueryVariables = Exact<{ [key: string]: never; }>;


export type NotificationSettingsQuery = { notificationSettings: { __typename: 'NotificationSettings', deployStarted: boolean | null, deploySucceeded: boolean | null, deployFailed: boolean | null } | null };

export type UpdateNotificationSettingsMutationVariables = Exact<{
  deployStarted: Scalars['Boolean']['input'];
  deploySucceeded: Scalars['Boolean']['input'];
  deployFailed: Scalars['Boolean']['input'];
}>;


export type UpdateNotificationSettingsMutation = { updateNotificationSettings: { __typename: 'NotificationSettings', deployStarted: boolean | null, deploySucceeded: boolean | null, deployFailed: boolean | null } | null };

export type ProjectFieldsFragment = { __typename: 'Project', id: string | null, name: string | null, ownerId: string | null, createdAt: string | null, serviceIds: Array<string | null> | null, databaseIds: Array<string | null> | null, keyValueIds: Array<string | null> | null };

export type ProjectsQueryVariables = Exact<{
  ownerId: Scalars['String']['input'];
}>;


export type ProjectsQuery = { projects: Array<{ __typename: 'Project', id: string | null, name: string | null, ownerId: string | null, createdAt: string | null, serviceIds: Array<string | null> | null, databaseIds: Array<string | null> | null, keyValueIds: Array<string | null> | null } | null> | null };

export type CreateProjectMutationVariables = Exact<{
  name: Scalars['String']['input'];
  ownerId: Scalars['String']['input'];
}>;


export type CreateProjectMutation = { createProject: { __typename: 'Project', id: string | null, name: string | null, ownerId: string | null, createdAt: string | null, serviceIds: Array<string | null> | null, databaseIds: Array<string | null> | null, keyValueIds: Array<string | null> | null } | null };

export type RenameProjectMutationVariables = Exact<{
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
}>;


export type RenameProjectMutation = { renameProject: { __typename: 'Project', id: string | null, name: string | null, ownerId: string | null, createdAt: string | null, serviceIds: Array<string | null> | null, databaseIds: Array<string | null> | null, keyValueIds: Array<string | null> | null } | null };

export type DeleteProjectMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DeleteProjectMutation = { deleteProject: string | null };

export type SetProjectServicesMutationVariables = Exact<{
  id: Scalars['String']['input'];
  serviceIds: Array<Scalars['String']['input']> | Scalars['String']['input'];
}>;


export type SetProjectServicesMutation = { setProjectServices: { __typename: 'Project', id: string | null, name: string | null, ownerId: string | null, createdAt: string | null, serviceIds: Array<string | null> | null, databaseIds: Array<string | null> | null, keyValueIds: Array<string | null> | null } | null };

export type SetProjectDatabasesMutationVariables = Exact<{
  id: Scalars['String']['input'];
  databaseIds: Array<Scalars['String']['input']> | Scalars['String']['input'];
}>;


export type SetProjectDatabasesMutation = { setProjectDatabases: { __typename: 'Project', id: string | null, name: string | null, ownerId: string | null, createdAt: string | null, serviceIds: Array<string | null> | null, databaseIds: Array<string | null> | null, keyValueIds: Array<string | null> | null } | null };

export type SetProjectKeyValuesMutationVariables = Exact<{
  id: Scalars['String']['input'];
  keyValueIds: Array<Scalars['String']['input']> | Scalars['String']['input'];
}>;


export type SetProjectKeyValuesMutation = { setProjectKeyValues: { __typename: 'Project', id: string | null, name: string | null, ownerId: string | null, createdAt: string | null, serviceIds: Array<string | null> | null, databaseIds: Array<string | null> | null, keyValueIds: Array<string | null> | null } | null };

export type RegistryCredentialsQueryVariables = Exact<{ [key: string]: never; }>;


export type RegistryCredentialsQuery = { registryCredentials: Array<{ __typename: 'RegistryCredential', id: string | null, name: string | null, host: string | null, username: string | null, expiresAt: string | null, status: string | null, createdAt: string | null } | null> | null };

export type CreateRegistryCredentialMutationVariables = Exact<{
  host: Scalars['String']['input'];
  username: Scalars['String']['input'];
  authToken: Scalars['String']['input'];
  name?: InputMaybe<Scalars['String']['input']>;
  expiresAt?: InputMaybe<Scalars['String']['input']>;
}>;


export type CreateRegistryCredentialMutation = { createRegistryCredential: { __typename: 'RegistryCredential', id: string | null, name: string | null, host: string | null, username: string | null, expiresAt: string | null, status: string | null, createdAt: string | null } | null };

export type DeleteRegistryCredentialMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DeleteRegistryCredentialMutation = { deleteRegistryCredential: boolean | null };

export type SetAutoDeployMutationVariables = Exact<{
  id: Scalars['String']['input'];
  enabled: Scalars['Boolean']['input'];
}>;


export type SetAutoDeployMutation = { setAutoDeploy: { __typename: 'Service', id: string | null, autoDeploy: boolean | null } | null };

export type AutoscalingConfigQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type AutoscalingConfigQuery = { autoscalingConfig: { __typename: 'Autoscaling', enabled: boolean | null, minInstances: number | null, maxInstances: number | null, targetCPUPercent: number | null, targetMemoryPercent: number | null } | null };

export type SetAutoscalingMutationVariables = Exact<{
  id: Scalars['String']['input'];
  minInstances: Scalars['Int']['input'];
  maxInstances: Scalars['Int']['input'];
  targetCPUPercent?: InputMaybe<Scalars['Int']['input']>;
  targetMemoryPercent?: InputMaybe<Scalars['Int']['input']>;
}>;


export type SetAutoscalingMutation = { setAutoscaling: { __typename: 'Autoscaling', enabled: boolean | null, minInstances: number | null, maxInstances: number | null, targetCPUPercent: number | null, targetMemoryPercent: number | null } | null };

export type DisableAutoscalingMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DisableAutoscalingMutation = { disableAutoscaling: boolean | null };

export type SetBuildFilterMutationVariables = Exact<{
  id: Scalars['String']['input'];
  buildFilter: BuildFilterInput;
}>;


export type SetBuildFilterMutation = { setBuildFilter: { __typename: 'Service', id: string | null, phase: string | null, buildFilter: { __typename: 'BuildFilter', paths: Array<string | null> | null, ignoredPaths: Array<string | null> | null } | null } | null };

export type SetBuildCommandMutationVariables = Exact<{
  id: Scalars['String']['input'];
  command: Scalars['String']['input'];
}>;


export type SetBuildCommandMutation = { setBuildCommand: { __typename: 'Service', id: string | null, buildCommand: string | null, phase: string | null } | null };

export type SetStartCommandMutationVariables = Exact<{
  id: Scalars['String']['input'];
  command: Scalars['String']['input'];
}>;


export type SetStartCommandMutation = { setStartCommand: { __typename: 'Service', id: string | null, startCommand: string | null, phase: string | null } | null };

export type SetDockerfilePathMutationVariables = Exact<{
  id: Scalars['String']['input'];
  dockerfilePath: Scalars['String']['input'];
}>;


export type SetDockerfilePathMutation = { setDockerfilePath: { __typename: 'Service', id: string | null, dockerfilePath: string | null, phase: string | null } | null };

export type CronJobRunsQueryVariables = Exact<{
  serviceId: Scalars['String']['input'];
  cursor?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
}>;


export type CronJobRunsQuery = { cronJobRuns: Array<{ __typename: 'CronRun', id: string | null, status: string | null, startedAt: string | null, finishedAt: string | null } | null> | null };

export type CancelCronJobRunMutationVariables = Exact<{
  serviceId: Scalars['String']['input'];
  runId: Scalars['String']['input'];
}>;


export type CancelCronJobRunMutation = { cancelCronJobRun: { __typename: 'CronRun', id: string | null, status: string | null, startedAt: string | null, finishedAt: string | null } | null };

export type CustomDomainsQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type CustomDomainsQuery = { customDomains: Array<{ __typename: 'CustomDomain', id: string | null, name: string | null, domainType: string | null, verificationStatus: string | null, serverStatus: string | null, redirectForName: string | null, dnsRecord: { __typename: 'DNSRecord', type: string | null, name: string | null, value: string | null } | null } | null> | null };

export type AddCustomDomainMutationVariables = Exact<{
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
}>;


export type AddCustomDomainMutation = { addCustomDomain: { __typename: 'CustomDomain', id: string | null, name: string | null, domainType: string | null, verificationStatus: string | null, serverStatus: string | null, redirectForName: string | null, dnsRecord: { __typename: 'DNSRecord', type: string | null, name: string | null, value: string | null } | null } | null };

export type DeleteCustomDomainMutationVariables = Exact<{
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
}>;


export type DeleteCustomDomainMutation = { deleteCustomDomain: boolean | null };

export type VerifyCustomDomainMutationVariables = Exact<{
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
}>;


export type VerifyCustomDomainMutation = { verifyCustomDomain: { __typename: 'CustomDomain', id: string | null, name: string | null, domainType: string | null, verificationStatus: string | null, serverStatus: string | null, redirectForName: string | null, dnsRecord: { __typename: 'DNSRecord', type: string | null, name: string | null, value: string | null } | null } | null };

export type DeployHookQueryVariables = Exact<{
  serviceId: Scalars['String']['input'];
}>;


export type DeployHookQuery = { deployHook: { __typename: 'DeployHook', url: string | null } | null };

export type RegenerateDeployHookMutationVariables = Exact<{
  serviceId: Scalars['String']['input'];
}>;


export type RegenerateDeployHookMutation = { regenerateDeployHook: { __typename: 'DeployHook', url: string | null } | null };

export type SetDisplayNameMutationVariables = Exact<{
  id: Scalars['String']['input'];
  displayName: Scalars['String']['input'];
}>;


export type SetDisplayNameMutation = { setDisplayName: { __typename: 'Service', id: string | null, name: string | null, displayName: string | null } | null };

export type EnvVarKeysQueryVariables = Exact<{
  serviceId: Scalars['String']['input'];
  cursor?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
}>;


export type EnvVarKeysQuery = { envVars: Array<{ __typename: 'EnvVarWithCursor', cursor: string | null, envVar: { __typename: 'EnvVarListValue', id: string | null, key: string | null } | null } | null> | null };

export type EnvVarValueQueryVariables = Exact<{
  id: Scalars['String']['input'];
  key: Scalars['String']['input'];
}>;


export type EnvVarValueQuery = { service: { __typename: 'Service', id: string | null, envVar: { __typename: 'EnvVar', id: string | null, key: string | null, value: string | null } | null } | null };

export type SetEnvVarsMutationVariables = Exact<{
  serviceId: Scalars['String']['input'];
  envVars: Array<EnvVarInput> | EnvVarInput;
}>;


export type SetEnvVarsMutation = { setEnvVars: boolean | null };

export type SetEnvVarMutationVariables = Exact<{
  serviceId: Scalars['String']['input'];
  key: Scalars['String']['input'];
  value?: InputMaybe<Scalars['String']['input']>;
  generateValue?: InputMaybe<Scalars['Boolean']['input']>;
}>;


export type SetEnvVarMutation = { setEnvVar: boolean | null };

export type DeleteEnvVarMutationVariables = Exact<{
  serviceId: Scalars['String']['input'];
  key: Scalars['String']['input'];
}>;


export type DeleteEnvVarMutation = { deleteEnvVar: boolean | null };

export type ServiceEventsQueryVariables = Exact<{
  serviceId: Scalars['String']['input'];
  cursor?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
}>;


export type ServiceEventsQuery = { serviceEvents: Array<{ __typename: 'ServiceEvent', id: string | null, type: string | null, timestamp: string | null, cursor: string | null, details: { __typename: 'ServiceEventDetails', deployId: string | null, deployStatus: string | null, preDeployStatus: string | null, actor: string | null, triggeredByUser: string | null, trigger: { __typename: 'DeployTrigger', firstBuild: boolean | null, envUpdated: boolean | null, manual: boolean | null, deployedByRender: boolean | null, clearCache: boolean | null, rollback: boolean | null } | null } | null } | null> | null };

export type TriggerDeployMutationVariables = Exact<{
  serviceId: Scalars['String']['input'];
  commitId?: InputMaybe<Scalars['String']['input']>;
  deployMode?: InputMaybe<Scalars['String']['input']>;
}>;


export type TriggerDeployMutation = { triggerDeploy: { __typename: 'Deploy', id: string | null, status: string | null, createdAt: string | null, trigger: string | null, rollbackOf: string | null, image: string | null } | null };

export type CancelDeployMutationVariables = Exact<{
  serviceId: Scalars['String']['input'];
  deployId: Scalars['String']['input'];
}>;


export type CancelDeployMutation = { cancelDeploy: { __typename: 'Deploy', id: string | null, status: string | null } | null };

export type RollbackServiceMutationVariables = Exact<{
  serviceId: Scalars['String']['input'];
  deployId: Scalars['String']['input'];
}>;


export type RollbackServiceMutation = { rollbackService: { __typename: 'Deploy', id: string | null, status: string | null, createdAt: string | null, trigger: string | null, rollbackOf: string | null, image: string | null } | null };

export type SetHealthCheckPathMutationVariables = Exact<{
  id: Scalars['String']['input'];
  path: Scalars['String']['input'];
}>;


export type SetHealthCheckPathMutation = { setHealthCheckPath: { __typename: 'Service', id: string | null, healthCheckPath: string | null } | null };

export type SetIdleTimeoutMutationVariables = Exact<{
  id: Scalars['String']['input'];
  idleTTLSeconds: Scalars['Int']['input'];
}>;


export type SetIdleTimeoutMutation = { setIdleTimeout: { __typename: 'Service', id: string | null, idleTTLSeconds: number | null, phase: string | null } | null };

export type InstanceTypesQueryVariables = Exact<{ [key: string]: never; }>;


export type InstanceTypesQuery = { instanceTypes: Array<{ __typename: 'InstanceType', id: string | null, name: string | null, cpu: string | null, memory: string | null } | null> | null };

export type UpdateServicePlanMutationVariables = Exact<{
  id: Scalars['String']['input'];
  plan: Scalars['String']['input'];
}>;


export type UpdateServicePlanMutation = { updateServicePlan: { __typename: 'Service', id: string | null, plan: string | null } | null };

export type SetServiceIpAllowListMutationVariables = Exact<{
  id: Scalars['String']['input'];
  cidrs?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>> | InputMaybe<Scalars['String']['input']>>;
}>;


export type SetServiceIpAllowListMutation = { setServiceIpAllowList: { __typename: 'Service', id: string | null, ipAllowList: Array<string | null> | null } | null };

export type SetMaintenanceModeMutationVariables = Exact<{
  id: Scalars['String']['input'];
  maintenanceMode: MaintenanceModeInput;
}>;


export type SetMaintenanceModeMutation = { setMaintenanceMode: { __typename: 'Service', id: string | null, maintenanceMode: { __typename: 'MaintenanceMode', enabled: boolean, uri: string } } | null };

export type SetMaxShutdownDelayMutationVariables = Exact<{
  id: Scalars['String']['input'];
  seconds: Scalars['Int']['input'];
}>;


export type SetMaxShutdownDelayMutation = { setMaxShutdownDelay: { __typename: 'Service', id: string | null, maxShutdownDelaySeconds: number | null } | null };

export type SetPreDeployCommandMutationVariables = Exact<{
  id: Scalars['String']['input'];
  command: Scalars['String']['input'];
}>;


export type SetPreDeployCommandMutation = { setPreDeployCommand: { __typename: 'Service', id: string | null, preDeployCommand: string | null, phase: string | null } | null };

export type SetRootDirMutationVariables = Exact<{
  id: Scalars['String']['input'];
  rootDir: Scalars['String']['input'];
}>;


export type SetRootDirMutation = { setRootDir: { __typename: 'Service', id: string | null, repo: string | null, branch: string | null, rootDir: string | null, phase: string | null } | null };

export type ScaleServiceMutationVariables = Exact<{
  id: Scalars['String']['input'];
  numInstances: Scalars['Int']['input'];
}>;


export type ScaleServiceMutation = { scaleService: { __typename: 'Service', id: string | null, replicas: number | null } | null };

export type SecretFileNamesQueryVariables = Exact<{
  serviceId: Scalars['String']['input'];
  cursor?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
}>;


export type SecretFileNamesQuery = { secretFiles: Array<{ __typename: 'SecretFileWithCursor', cursor: string | null, secretFile: { __typename: 'SecretFileListValue', id: string | null, name: string | null } | null } | null> | null };

export type SecretFileContentQueryVariables = Exact<{
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
}>;


export type SecretFileContentQuery = { service: { __typename: 'Service', id: string | null, secretFile: { __typename: 'SecretFile', id: string | null, name: string | null, content: string | null } | null } | null };

export type SetSecretFileMutationVariables = Exact<{
  serviceId: Scalars['String']['input'];
  name: Scalars['String']['input'];
  content?: InputMaybe<Scalars['String']['input']>;
}>;


export type SetSecretFileMutation = { setSecretFile: boolean | null };

export type DeleteSecretFileMutationVariables = Exact<{
  serviceId: Scalars['String']['input'];
  name: Scalars['String']['input'];
}>;


export type DeleteSecretFileMutation = { deleteSecretFile: boolean | null };

export type ServerQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type ServerQuery = { server: { __typename: 'Service', id: string | null, name: string | null, slug: string | null, displayName: string | null, type: string | null, suspended: string | null, dashboardUrl: string | null, url: string | null, createdAt: string | null, sshAddress: string | null, phase: string | null, replicas: number | null, revision: string | null, plan: string | null, idleTTLSeconds: number | null, repo: string | null, branch: string | null, rootDir: string | null, runtime: string | null, builder: string | null, buildCommand: string | null, startCommand: string | null, dockerfilePath: string | null, registryCredentialId: string | null, autoDeploy: boolean | null, notifyOnFail: string | null, notificationsToSend: string | null, renderSubdomainPolicy: string | null, healthCheckPath: string | null, maxShutdownDelaySeconds: number | null, preDeployCommand: string | null, schedule: string | null, command: string | null, lastSuccessfulRunAt: string | null, publishPath: string | null, ipAllowList: Array<string | null> | null, maintenanceMode: { __typename: 'MaintenanceMode', enabled: boolean, uri: string }, buildFilter: { __typename: 'BuildFilter', paths: Array<string | null> | null, ignoredPaths: Array<string | null> | null } | null, runs: Array<{ __typename: 'CronRun', id: string | null, name: string | null, startedAt: string | null, finishedAt: string | null, status: string | null } | null> | null, routes: Array<{ __typename: 'StaticRoute', type: string | null, source: string | null, destination: string | null } | null> | null, headers: Array<{ __typename: 'StaticHeader', path: string | null, name: string | null, value: string | null } | null> | null } | null };

export type SetNotificationsToSendMutationVariables = Exact<{
  id: Scalars['String']['input'];
  value: Scalars['String']['input'];
}>;


export type SetNotificationsToSendMutation = { setNotificationsToSend: { __typename: 'Service', id: string | null, notificationsToSend: string | null, notifyOnFail: string | null } | null };

export type ServicesQueryVariables = Exact<{
  ownerId?: InputMaybe<Scalars['String']['input']>;
}>;


export type ServicesQuery = { services: Array<{ __typename: 'Service', id: string | null, name: string | null, displayName: string | null, type: string | null, suspended: string | null, dashboardUrl: string | null, url: string | null, createdAt: string | null, phase: string | null, replicas: number | null, revision: string | null, plan: string | null, idleTTLSeconds: number | null } | null> | null };

export type SuspendServiceMutationVariables = Exact<{
  id: Scalars['String']['input'];
  confirm?: InputMaybe<Scalars['String']['input']>;
}>;


export type SuspendServiceMutation = { suspendService: { __typename: 'Service', id: string | null, suspended: string | null, phase: string | null } | null };

export type ResumeServiceMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type ResumeServiceMutation = { resumeService: { __typename: 'Service', id: string | null, suspended: string | null, phase: string | null } | null };

export type DeleteServiceMutationVariables = Exact<{
  id: Scalars['String']['input'];
  confirm?: InputMaybe<Scalars['String']['input']>;
}>;


export type DeleteServiceMutation = { deleteService: boolean | null };

export type UpdateCronJobMutationVariables = Exact<{
  id: Scalars['String']['input'];
  schedule: Scalars['String']['input'];
  command?: InputMaybe<Scalars['String']['input']>;
}>;


export type UpdateCronJobMutation = { updateCronJob: { __typename: 'Service', id: string | null, schedule: string | null, command: string | null } | null };

export type CreateServiceMutationVariables = Exact<{
  name: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
  environmentId?: InputMaybe<Scalars['String']['input']>;
  type?: InputMaybe<Scalars['String']['input']>;
  repo?: InputMaybe<Scalars['String']['input']>;
  image?: InputMaybe<Scalars['String']['input']>;
  registryCredentialId?: InputMaybe<Scalars['String']['input']>;
  branch?: InputMaybe<Scalars['String']['input']>;
  rootDir?: InputMaybe<Scalars['String']['input']>;
  runtime?: InputMaybe<Scalars['String']['input']>;
  buildCommand?: InputMaybe<Scalars['String']['input']>;
  startCommand?: InputMaybe<Scalars['String']['input']>;
  dockerfilePath?: InputMaybe<Scalars['String']['input']>;
  buildFilter?: InputMaybe<BuildFilterInput>;
  plan?: InputMaybe<Scalars['String']['input']>;
  autoDeploy?: InputMaybe<Scalars['Boolean']['input']>;
  schedule?: InputMaybe<Scalars['String']['input']>;
  command?: InputMaybe<Scalars['String']['input']>;
  publishPath?: InputMaybe<Scalars['String']['input']>;
  envVars?: InputMaybe<Array<InputMaybe<EnvVarInput>> | InputMaybe<EnvVarInput>>;
  secretFiles?: InputMaybe<Array<InputMaybe<SecretFileInput>> | InputMaybe<SecretFileInput>>;
}>;


export type CreateServiceMutation = { createService: { __typename: 'Service', id: string | null, name: string | null, type: string | null, phase: string | null, projectId: string | null, environmentId: string | null, registryCredentialId: string | null, latestDeployId: string | null } | null };

export type SetRegistryCredentialMutationVariables = Exact<{
  id: Scalars['String']['input'];
  registryCredentialId: Scalars['String']['input'];
}>;


export type SetRegistryCredentialMutation = { setRegistryCredential: { __typename: 'Service', id: string | null, registryCredentialId: string | null } | null };

export type ServiceNameAvailableQueryVariables = Exact<{
  name: Scalars['String']['input'];
}>;


export type ServiceNameAvailableQuery = { serviceNameAvailable: { __typename: 'NameAvailability', available: boolean | null, suggestion: string | null } | null };

export type SetStaticRoutesMutationVariables = Exact<{
  id: Scalars['String']['input'];
  routes?: InputMaybe<Array<InputMaybe<StaticRouteInput>> | InputMaybe<StaticRouteInput>>;
}>;


export type SetStaticRoutesMutation = { setStaticRoutes: { __typename: 'Service', id: string | null, routes: Array<{ __typename: 'StaticRoute', type: string | null, source: string | null, destination: string | null } | null> | null } | null };

export type SetStaticHeadersMutationVariables = Exact<{
  id: Scalars['String']['input'];
  headers?: InputMaybe<Array<InputMaybe<StaticHeaderInput>> | InputMaybe<StaticHeaderInput>>;
}>;


export type SetStaticHeadersMutation = { setStaticHeaders: { __typename: 'Service', id: string | null, headers: Array<{ __typename: 'StaticHeader', path: string | null, name: string | null, value: string | null } | null> | null } | null };

export type SetPublishPathMutationVariables = Exact<{
  id: Scalars['String']['input'];
  publishPath: Scalars['String']['input'];
}>;


export type SetPublishPathMutation = { setPublishPath: { __typename: 'Service', id: string | null, publishPath: string | null, revision: string | null } | null };

export type SetSubdomainPolicyMutationVariables = Exact<{
  id: Scalars['String']['input'];
  policy: Scalars['String']['input'];
}>;


export type SetSubdomainPolicyMutation = { setSubdomainPolicy: { __typename: 'Service', id: string | null, renderSubdomainPolicy: string | null } | null };

export type SshKeysQueryVariables = Exact<{ [key: string]: never; }>;


export type SshKeysQuery = { sshKeys: Array<{ __typename: 'SSHKey', id: string, name: string, publicKey: string, fingerprint: string, createdAt: string } | null> | null };

export type CreateSshKeyMutationVariables = Exact<{
  name: Scalars['String']['input'];
  publicKey: Scalars['String']['input'];
}>;


export type CreateSshKeyMutation = { createSSHKey: { __typename: 'SSHKey', id: string, name: string, publicKey: string, fingerprint: string, createdAt: string } | null };

export type DeleteSshKeyMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DeleteSshKeyMutation = { deleteSSHKey: boolean };

export type WorkspaceMembersQueryVariables = Exact<{
  workspaceId: Scalars['String']['input'];
}>;


export type WorkspaceMembersQuery = { workspaceMembers: Array<{ __typename: 'WorkspaceMember', subject: string | null, userId: string | null, email: string | null, role: string | null, createdAt: string | null, mfaEnabled: boolean | null } | null> | null, workspaceSeatUsage: { __typename: 'WorkspaceSeatUsage', used: number | null, limit: number | null } | null };

export type WorkspaceInvitesQueryVariables = Exact<{
  workspaceId: Scalars['String']['input'];
}>;


export type WorkspaceInvitesQuery = { workspaceInvites: Array<{ __typename: 'WorkspaceInvite', id: string | null, email: string | null, role: string | null, expiresAt: string | null, createdAt: string | null } | null> | null };

export type InviteWorkspaceMemberMutationVariables = Exact<{
  workspaceId: Scalars['String']['input'];
  email: Scalars['String']['input'];
  role: Scalars['String']['input'];
}>;


export type InviteWorkspaceMemberMutation = { inviteWorkspaceMember: { __typename: 'WorkspaceInvite', id: string | null, email: string | null, role: string | null, expiresAt: string | null, createdAt: string | null } | null };

export type ChangeWorkspaceMemberRoleMutationVariables = Exact<{
  workspaceId: Scalars['String']['input'];
  subject: Scalars['String']['input'];
  role: Scalars['String']['input'];
}>;


export type ChangeWorkspaceMemberRoleMutation = { changeWorkspaceMemberRole: { __typename: 'WorkspaceMember', subject: string | null, role: string | null, createdAt: string | null } | null };

export type RemoveWorkspaceMemberMutationVariables = Exact<{
  workspaceId: Scalars['String']['input'];
  subject: Scalars['String']['input'];
}>;


export type RemoveWorkspaceMemberMutation = { removeWorkspaceMember: string | null };

export type RevokeWorkspaceInviteMutationVariables = Exact<{
  workspaceId: Scalars['String']['input'];
  inviteId: Scalars['String']['input'];
}>;


export type RevokeWorkspaceInviteMutation = { revokeWorkspaceInvite: string | null };

export type ResendWorkspaceInviteMutationVariables = Exact<{
  workspaceId: Scalars['String']['input'];
  inviteId: Scalars['String']['input'];
}>;


export type ResendWorkspaceInviteMutation = { resendWorkspaceInvite: { __typename: 'WorkspaceInvite', id: string | null, email: string | null, role: string | null, expiresAt: string | null, createdAt: string | null } | null };

export type AcceptWorkspaceInviteMutationVariables = Exact<{
  token: Scalars['String']['input'];
}>;


export type AcceptWorkspaceInviteMutation = { acceptWorkspaceInvite: { __typename: 'AcceptedWorkspaceInvite', workspaceId: string | null, workspaceName: string | null, role: string | null } | null };

export type UsageQueryVariables = Exact<{
  period?: InputMaybe<Scalars['String']['input']>;
  ownerId?: InputMaybe<Scalars['String']['input']>;
}>;


export type UsageQuery = { usage: { __typename: 'UsageSummary', workspaceId: string | null, period: string | null, services: Array<{ __typename: 'ServiceUsage', serviceId: string | null, resourceKind: string | null, rows: Array<{ __typename: 'UsageRow', kind: string | null, tier: string | null, total: number | null } | null> | null } | null> | null, estimatedCost: { __typename: 'EstimatedCost', totalUsd: string | null, meters: Array<{ __typename: 'MeterEstimate', kind: string | null, tier: string | null, resourceKind: string | null, costUsd: string | null } | null> | null } | null } | null };

export type WorkspaceLimitsQueryVariables = Exact<{
  ownerId: Scalars['String']['input'];
}>;


export type WorkspaceLimitsQuery = { workspaceLimits: { __typename: 'ResourceLimits', services: { __typename: 'ResourceCap', used: number | null, limit: number | null } | null, postgres: { __typename: 'ResourceCap', used: number | null, limit: number | null } | null, keyValues: { __typename: 'ResourceCap', used: number | null, limit: number | null } | null } | null };

export type WebhookEndpointsQueryVariables = Exact<{
  ownerId?: InputMaybe<Scalars['String']['input']>;
}>;


export type WebhookEndpointsQuery = { webhookEndpoints: Array<{ __typename: 'WebhookEndpoint', id: string | null, name: string | null, url: string | null, eventTypes: Array<string | null> | null, enabled: boolean | null, disabledReason: string | null, createdAt: string | null } | null> | null };

export type WebhookEventTypesQueryVariables = Exact<{ [key: string]: never; }>;


export type WebhookEventTypesQuery = { webhookEventTypes: Array<string | null> | null };

export type WebhookDeliveriesQueryVariables = Exact<{
  endpointId: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
  cursor?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
}>;


export type WebhookDeliveriesQuery = { webhookDeliveries: Array<{ __typename: 'WebhookDelivery', id: string | null, eventType: string | null, serviceId: string | null, status: string | null, attemptCount: number | null, lastStatusCode: number | null, lastError: string | null, nextAttemptAt: string | null, deliveredAt: string | null, createdAt: string | null, cursor: string | null } | null> | null };

export type CreateWebhookEndpointMutationVariables = Exact<{
  ownerId?: InputMaybe<Scalars['String']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  url: Scalars['String']['input'];
  eventTypes: Array<InputMaybe<Scalars['String']['input']>> | InputMaybe<Scalars['String']['input']>;
}>;


export type CreateWebhookEndpointMutation = { createWebhookEndpoint: { __typename: 'WebhookEndpoint', id: string | null, name: string | null, url: string | null, eventTypes: Array<string | null> | null, enabled: boolean | null, secret: string | null, createdAt: string | null } | null };

export type SetWebhookEndpointEnabledMutationVariables = Exact<{
  id: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
  enabled: Scalars['Boolean']['input'];
}>;


export type SetWebhookEndpointEnabledMutation = { setWebhookEndpointEnabled: { __typename: 'WebhookEndpoint', id: string | null, enabled: boolean | null, disabledReason: string | null } | null };

export type DeleteWebhookEndpointMutationVariables = Exact<{
  id: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
}>;


export type DeleteWebhookEndpointMutation = { deleteWebhookEndpoint: boolean | null };

export type WorkspacesQueryVariables = Exact<{ [key: string]: never; }>;


export type WorkspacesQuery = { workspaces: Array<{ __typename: 'Workspace', id: string | null, name: string | null, plan: string | null, role: string | null, createdAt: string | null } | null> | null };

export type CreateWorkspaceMutationVariables = Exact<{
  name: Scalars['String']['input'];
  plan?: InputMaybe<Scalars['String']['input']>;
}>;


export type CreateWorkspaceMutation = { createWorkspace: { __typename: 'Workspace', id: string | null, name: string | null, plan: string | null, role: string | null, createdAt: string | null } | null };

export type RenameWorkspaceMutationVariables = Exact<{
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
}>;


export type RenameWorkspaceMutation = { renameWorkspace: { __typename: 'Workspace', id: string | null, name: string | null, plan: string | null, role: string | null, createdAt: string | null } | null };

export type ChangeWorkspacePlanMutationVariables = Exact<{
  id: Scalars['String']['input'];
  plan: Scalars['String']['input'];
}>;


export type ChangeWorkspacePlanMutation = { changeWorkspacePlan: { __typename: 'Workspace', id: string | null, name: string | null, plan: string | null, role: string | null, createdAt: string | null } | null };

export type DeleteWorkspaceMutationVariables = Exact<{
  id: Scalars['String']['input'];
  confirmation: Scalars['String']['input'];
}>;


export type DeleteWorkspaceMutation = { deleteWorkspace: string | null };

export const EnvironmentFieldsFragmentDoc = {"kind":"Document","definitions":[{"kind":"FragmentDefinition","name":{"kind":"Name","value":"EnvironmentFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Environment"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"projectId"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"ownerId"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"serviceIds"}},{"kind":"Field","name":{"kind":"Name","value":"databaseIds"}},{"kind":"Field","name":{"kind":"Name","value":"keyValueIds"}},{"kind":"Field","name":{"kind":"Name","value":"envGroupIds"}},{"kind":"Field","name":{"kind":"Name","value":"protectedStatus"}},{"kind":"Field","name":{"kind":"Name","value":"networkIsolationEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowList"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowListEntries"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"cidrBlock"}},{"kind":"Field","name":{"kind":"Name","value":"description"}}]}}]}}]} as unknown as DocumentNode<EnvironmentFieldsFragment, unknown>;
export const ProjectFieldsFragmentDoc = {"kind":"Document","definitions":[{"kind":"FragmentDefinition","name":{"kind":"Name","value":"ProjectFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Project"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"ownerId"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"serviceIds"}},{"kind":"Field","name":{"kind":"Name","value":"databaseIds"}},{"kind":"Field","name":{"kind":"Name","value":"keyValueIds"}}]}}]} as unknown as DocumentNode<ProjectFieldsFragment, unknown>;
export const ApiKeysDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"ApiKeys"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"apiKeys"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"createdBy"}},{"kind":"Field","name":{"kind":"Name","value":"lastUsedAt"}}]}}]}}]} as unknown as DocumentNode<ApiKeysQuery, ApiKeysQueryVariables>;
export const CreateApiKeyDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateApiKey"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createApiKey"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"secret"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<CreateApiKeyMutation, CreateApiKeyMutationVariables>;
export const RevokeApiKeyDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RevokeApiKey"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"revokeApiKey"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}}]}]}}]} as unknown as DocumentNode<RevokeApiKeyMutation, RevokeApiKeyMutationVariables>;
export const AuditLogsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"AuditLogs"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"startTime"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"endTime"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"cursor"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"limit"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"auditLogs"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}},{"kind":"Argument","name":{"kind":"Name","value":"startTime"},"value":{"kind":"Variable","name":{"kind":"Name","value":"startTime"}}},{"kind":"Argument","name":{"kind":"Name","value":"endTime"},"value":{"kind":"Variable","name":{"kind":"Name","value":"endTime"}}},{"kind":"Argument","name":{"kind":"Name","value":"cursor"},"value":{"kind":"Variable","name":{"kind":"Name","value":"cursor"}}},{"kind":"Argument","name":{"kind":"Name","value":"limit"},"value":{"kind":"Variable","name":{"kind":"Name","value":"limit"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"timestamp"}},{"kind":"Field","name":{"kind":"Name","value":"actor"}},{"kind":"Field","name":{"kind":"Name","value":"actorMethod"}},{"kind":"Field","name":{"kind":"Name","value":"action"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"resource"}},{"kind":"Field","name":{"kind":"Name","value":"targetName"}}]}}]}}]} as unknown as DocumentNode<AuditLogsQuery, AuditLogsQueryVariables>;
export const DatabasesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Databases"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databases"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"version"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"diskSizeGB"}},{"kind":"Field","name":{"kind":"Name","value":"diskAutoscalingEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"public"}}]}}]}}]} as unknown as DocumentNode<DatabasesQuery, DatabasesQueryVariables>;
export const DatabaseDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Database"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"database"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"version"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"databaseName"}},{"kind":"Field","name":{"kind":"Name","value":"databaseUser"}},{"kind":"Field","name":{"kind":"Name","value":"diskSizeGB"}},{"kind":"Field","name":{"kind":"Name","value":"diskAutoscalingEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"highAvailabilityEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"readReplicas"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"connectionInfo"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"internalHost"}},{"kind":"Field","name":{"kind":"Name","value":"externalHost"}}]}}]}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"externalHost"}},{"kind":"Field","name":{"kind":"Name","value":"public"}},{"kind":"Field","name":{"kind":"Name","value":"poolerEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"backupsEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowList"}}]}}]}}]} as unknown as DocumentNode<DatabaseQuery, DatabaseQueryVariables>;
export const DatabaseConnectionInfoDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseConnectionInfo"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseConnectionInfo"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"password"}},{"kind":"Field","name":{"kind":"Name","value":"internalConnectionString"}},{"kind":"Field","name":{"kind":"Name","value":"externalConnectionString"}},{"kind":"Field","name":{"kind":"Name","value":"internalConnectionPoolString"}},{"kind":"Field","name":{"kind":"Name","value":"externalConnectionPoolString"}},{"kind":"Field","name":{"kind":"Name","value":"psqlCommand"}},{"kind":"Field","name":{"kind":"Name","value":"readReplicaConnectionStrings"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"internalConnectionString"}},{"kind":"Field","name":{"kind":"Name","value":"externalConnectionString"}}]}}]}}]}}]} as unknown as DocumentNode<DatabaseConnectionInfoQuery, DatabaseConnectionInfoQueryVariables>;
export const DatabaseRecoveryInfoDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseRecoveryInfo"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseRecoveryInfo"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"enabled"}},{"kind":"Field","name":{"kind":"Name","value":"earliestRecoveryTime"}},{"kind":"Field","name":{"kind":"Name","value":"latestRecoveryTime"}},{"kind":"Field","name":{"kind":"Name","value":"backups"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]}}]} as unknown as DocumentNode<DatabaseRecoveryInfoQuery, DatabaseRecoveryInfoQueryVariables>;
export const DatabaseExportsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseExports"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseExports"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"url"}},{"kind":"Field","name":{"kind":"Name","value":"urlExpiresAt"}},{"kind":"Field","name":{"kind":"Name","value":"expiresAt"}},{"kind":"Field","name":{"kind":"Name","value":"filename"}},{"kind":"Field","name":{"kind":"Name","value":"failureReason"}}]}}]}}]} as unknown as DocumentNode<DatabaseExportsQuery, DatabaseExportsQueryVariables>;
export const DatabaseUsersDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseUsers"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseUsers"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}}]}}]}}]} as unknown as DocumentNode<DatabaseUsersQuery, DatabaseUsersQueryVariables>;
export const DatabaseIpAllowListDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseIpAllowList"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"database"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowListEntries"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"cidrBlock"}},{"kind":"Field","name":{"kind":"Name","value":"description"}}]}}]}}]}}]} as unknown as DocumentNode<DatabaseIpAllowListQuery, DatabaseIpAllowListQueryVariables>;
export const FailoverDatabaseDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"FailoverDatabase"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"failoverDatabase"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<FailoverDatabaseMutation, FailoverDatabaseMutationVariables>;
export const SuspendDatabaseDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SuspendDatabase"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"suspendDatabase"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"status"}}]}}]}}]} as unknown as DocumentNode<SuspendDatabaseMutation, SuspendDatabaseMutationVariables>;
export const ResumeDatabaseDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ResumeDatabase"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"resumeDatabase"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"status"}}]}}]}}]} as unknown as DocumentNode<ResumeDatabaseMutation, ResumeDatabaseMutationVariables>;
export const RestartDatabaseDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RestartDatabase"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"restartDatabase"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"status"}}]}}]}}]} as unknown as DocumentNode<RestartDatabaseMutation, RestartDatabaseMutationVariables>;
export const RecoverDatabaseDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RecoverDatabase"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"targetTime"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"plan"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"version"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"recoverDatabase"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"targetTime"},"value":{"kind":"Variable","name":{"kind":"Name","value":"targetTime"}}},{"kind":"Argument","name":{"kind":"Name","value":"plan"},"value":{"kind":"Variable","name":{"kind":"Name","value":"plan"}}},{"kind":"Argument","name":{"kind":"Name","value":"version"},"value":{"kind":"Variable","name":{"kind":"Name","value":"version"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"status"}}]}}]}}]} as unknown as DocumentNode<RecoverDatabaseMutation, RecoverDatabaseMutationVariables>;
export const CreateDatabaseExportDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateDatabaseExport"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createDatabaseExport"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"url"}},{"kind":"Field","name":{"kind":"Name","value":"urlExpiresAt"}},{"kind":"Field","name":{"kind":"Name","value":"expiresAt"}},{"kind":"Field","name":{"kind":"Name","value":"filename"}},{"kind":"Field","name":{"kind":"Name","value":"failureReason"}}]}}]}}]} as unknown as DocumentNode<CreateDatabaseExportMutation, CreateDatabaseExportMutationVariables>;
export const SetDatabaseIpAllowListDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetDatabaseIpAllowList"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"entries"}},"type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"IPAllowListEntryInput"}}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setDatabaseIpAllowList"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"entries"},"value":{"kind":"Variable","name":{"kind":"Name","value":"entries"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowListEntries"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"cidrBlock"}},{"kind":"Field","name":{"kind":"Name","value":"description"}}]}}]}}]}}]} as unknown as DocumentNode<SetDatabaseIpAllowListMutation, SetDatabaseIpAllowListMutationVariables>;
export const CreateDatabaseUserDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateDatabaseUser"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createDatabaseUser"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"password"}}]}}]}}]} as unknown as DocumentNode<CreateDatabaseUserMutation, CreateDatabaseUserMutationVariables>;
export const DeleteDatabaseUserDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteDatabaseUser"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteDatabaseUser"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}]}]}}]} as unknown as DocumentNode<DeleteDatabaseUserMutation, DeleteDatabaseUserMutationVariables>;
export const DatabaseInstanceTypesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseInstanceTypes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseInstanceTypes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"cpu"}},{"kind":"Field","name":{"kind":"Name","value":"memory"}},{"kind":"Field","name":{"kind":"Name","value":"storageGB"}}]}}]}}]} as unknown as DocumentNode<DatabaseInstanceTypesQuery, DatabaseInstanceTypesQueryVariables>;
export const CreateDatabaseDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateDatabase"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"environmentId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"plan"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"version"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"diskSizeGB"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"public"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createDatabase"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}},{"kind":"Argument","name":{"kind":"Name","value":"environmentId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"environmentId"}}},{"kind":"Argument","name":{"kind":"Name","value":"plan"},"value":{"kind":"Variable","name":{"kind":"Name","value":"plan"}}},{"kind":"Argument","name":{"kind":"Name","value":"version"},"value":{"kind":"Variable","name":{"kind":"Name","value":"version"}}},{"kind":"Argument","name":{"kind":"Name","value":"diskSizeGB"},"value":{"kind":"Variable","name":{"kind":"Name","value":"diskSizeGB"}}},{"kind":"Argument","name":{"kind":"Name","value":"public"},"value":{"kind":"Variable","name":{"kind":"Name","value":"public"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"projectId"}},{"kind":"Field","name":{"kind":"Name","value":"environmentId"}}]}}]}}]} as unknown as DocumentNode<CreateDatabaseMutation, CreateDatabaseMutationVariables>;
export const UpdateDatabasePlanDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"UpdateDatabasePlan"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"plan"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"updateDatabasePlan"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"plan"},"value":{"kind":"Variable","name":{"kind":"Name","value":"plan"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"status"}}]}}]}}]} as unknown as DocumentNode<UpdateDatabasePlanMutation, UpdateDatabasePlanMutationVariables>;
export const UpdateDatabaseVersionDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"UpdateDatabaseVersion"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"version"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"updateDatabaseVersion"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"version"},"value":{"kind":"Variable","name":{"kind":"Name","value":"version"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"version"}},{"kind":"Field","name":{"kind":"Name","value":"status"}}]}}]}}]} as unknown as DocumentNode<UpdateDatabaseVersionMutation, UpdateDatabaseVersionMutationVariables>;
export const UpdateDatabaseDiskAutoscalingDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"UpdateDatabaseDiskAutoscaling"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"enabled"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"updateDatabaseDiskAutoscaling"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"enabled"},"value":{"kind":"Variable","name":{"kind":"Name","value":"enabled"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"diskSizeGB"}},{"kind":"Field","name":{"kind":"Name","value":"diskAutoscalingEnabled"}}]}}]}}]} as unknown as DocumentNode<UpdateDatabaseDiskAutoscalingMutation, UpdateDatabaseDiskAutoscalingMutationVariables>;
export const RenameDatabaseDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RenameDatabase"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"renameDatabase"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"databaseName"}},{"kind":"Field","name":{"kind":"Name","value":"databaseUser"}},{"kind":"Field","name":{"kind":"Name","value":"externalHost"}}]}}]}}]} as unknown as DocumentNode<RenameDatabaseMutation, RenameDatabaseMutationVariables>;
export const DeleteDatabaseDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteDatabase"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteDatabase"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<DeleteDatabaseMutation, DeleteDatabaseMutationVariables>;
export const DatabaseProcessesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseProcesses"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseProcesses"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"pid"}},{"kind":"Field","name":{"kind":"Name","value":"userName"}},{"kind":"Field","name":{"kind":"Name","value":"applicationName"}},{"kind":"Field","name":{"kind":"Name","value":"state"}},{"kind":"Field","name":{"kind":"Name","value":"query"}},{"kind":"Field","name":{"kind":"Name","value":"waitEventType"}},{"kind":"Field","name":{"kind":"Name","value":"waitEvent"}},{"kind":"Field","name":{"kind":"Name","value":"durationSeconds"}}]}}]}}]} as unknown as DocumentNode<DatabaseProcessesQuery, DatabaseProcessesQueryVariables>;
export const DatabaseTopQueriesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseTopQueries"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseTopQueries"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"query"}},{"kind":"Field","name":{"kind":"Name","value":"calls"}},{"kind":"Field","name":{"kind":"Name","value":"totalTimeMs"}},{"kind":"Field","name":{"kind":"Name","value":"meanTimeMs"}},{"kind":"Field","name":{"kind":"Name","value":"rows"}},{"kind":"Field","name":{"kind":"Name","value":"sharedHitBlks"}},{"kind":"Field","name":{"kind":"Name","value":"sharedReadBlks"}}]}}]}}]} as unknown as DocumentNode<DatabaseTopQueriesQuery, DatabaseTopQueriesQueryVariables>;
export const DatabaseSizesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseSizes"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseSizes"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"database"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"sizeBytes"}},{"kind":"Field","name":{"kind":"Name","value":"sizePretty"}}]}},{"kind":"Field","name":{"kind":"Name","value":"tables"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"schema"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"sizeBytes"}},{"kind":"Field","name":{"kind":"Name","value":"sizePretty"}}]}}]}}]}}]} as unknown as DocumentNode<DatabaseSizesQuery, DatabaseSizesQueryVariables>;
export const DatabaseTableScansDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseTableScans"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseTableScans"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"schema"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"seqScans"}},{"kind":"Field","name":{"kind":"Name","value":"seqScanRows"}},{"kind":"Field","name":{"kind":"Name","value":"indexScans"}},{"kind":"Field","name":{"kind":"Name","value":"indexScanRows"}},{"kind":"Field","name":{"kind":"Name","value":"liveRows"}},{"kind":"Field","name":{"kind":"Name","value":"deadRows"}}]}}]}}]} as unknown as DocumentNode<DatabaseTableScansQuery, DatabaseTableScansQueryVariables>;
export const DatabaseParameterOverridesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseParameterOverrides"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseParameterOverrides"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"setting"}},{"kind":"Field","name":{"kind":"Name","value":"unit"}},{"kind":"Field","name":{"kind":"Name","value":"source"}},{"kind":"Field","name":{"kind":"Name","value":"description"}}]}}]}}]} as unknown as DocumentNode<DatabaseParameterOverridesQuery, DatabaseParameterOverridesQueryVariables>;
export const SetDatabaseParameterOverridesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetDatabaseParameterOverrides"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"parameters"}},"type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ParameterInput"}}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setDatabaseParameterOverrides"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"parameters"},"value":{"kind":"Variable","name":{"kind":"Name","value":"parameters"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}}]}}]}}]} as unknown as DocumentNode<SetDatabaseParameterOverridesMutation, SetDatabaseParameterOverridesMutationVariables>;
export const ExecuteDatabaseQueryDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ExecuteDatabaseQuery"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"sql"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"allowWrites"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"executeDatabaseQuery"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"sql"},"value":{"kind":"Variable","name":{"kind":"Name","value":"sql"}}},{"kind":"Argument","name":{"kind":"Name","value":"allowWrites"},"value":{"kind":"Variable","name":{"kind":"Name","value":"allowWrites"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"columns"}},{"kind":"Field","name":{"kind":"Name","value":"rows"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"values"}}]}},{"kind":"Field","name":{"kind":"Name","value":"rowCount"}},{"kind":"Field","name":{"kind":"Name","value":"truncated"}}]}}]}}]} as unknown as DocumentNode<ExecuteDatabaseQueryMutation, ExecuteDatabaseQueryMutationVariables>;
export const DeployDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Deploy"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"deployId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deploy"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"deployId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"deployId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"trigger"}},{"kind":"Field","name":{"kind":"Name","value":"image"}},{"kind":"Field","name":{"kind":"Name","value":"rollbackOf"}},{"kind":"Field","name":{"kind":"Name","value":"commitId"}},{"kind":"Field","name":{"kind":"Name","value":"commitMessage"}},{"kind":"Field","name":{"kind":"Name","value":"commitCreatedAt"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"updatedAt"}},{"kind":"Field","name":{"kind":"Name","value":"startedAt"}},{"kind":"Field","name":{"kind":"Name","value":"finishedAt"}},{"kind":"Field","name":{"kind":"Name","value":"preDeployStatus"}}]}}]}}]} as unknown as DocumentNode<DeployQuery, DeployQueryVariables>;
export const DeployTimelineEventsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DeployTimelineEvents"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"startTime"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"endTime"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"limit"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"serviceEvents"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"startTime"},"value":{"kind":"Variable","name":{"kind":"Name","value":"startTime"}}},{"kind":"Argument","name":{"kind":"Name","value":"endTime"},"value":{"kind":"Variable","name":{"kind":"Name","value":"endTime"}}},{"kind":"Argument","name":{"kind":"Name","value":"limit"},"value":{"kind":"Variable","name":{"kind":"Name","value":"limit"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"timestamp"}},{"kind":"Field","name":{"kind":"Name","value":"details"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deployId"}},{"kind":"Field","name":{"kind":"Name","value":"deployStatus"}}]}}]}}]}}]} as unknown as DocumentNode<DeployTimelineEventsQuery, DeployTimelineEventsQueryVariables>;
export const DeploysDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Deploys"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"status"}},"type":{"kind":"ListType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"createdBefore"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"createdAfter"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"updatedBefore"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"updatedAfter"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"finishedBefore"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"finishedAfter"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"cursor"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"limit"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deploys"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"status"},"value":{"kind":"Variable","name":{"kind":"Name","value":"status"}}},{"kind":"Argument","name":{"kind":"Name","value":"createdBefore"},"value":{"kind":"Variable","name":{"kind":"Name","value":"createdBefore"}}},{"kind":"Argument","name":{"kind":"Name","value":"createdAfter"},"value":{"kind":"Variable","name":{"kind":"Name","value":"createdAfter"}}},{"kind":"Argument","name":{"kind":"Name","value":"updatedBefore"},"value":{"kind":"Variable","name":{"kind":"Name","value":"updatedBefore"}}},{"kind":"Argument","name":{"kind":"Name","value":"updatedAfter"},"value":{"kind":"Variable","name":{"kind":"Name","value":"updatedAfter"}}},{"kind":"Argument","name":{"kind":"Name","value":"finishedBefore"},"value":{"kind":"Variable","name":{"kind":"Name","value":"finishedBefore"}}},{"kind":"Argument","name":{"kind":"Name","value":"finishedAfter"},"value":{"kind":"Variable","name":{"kind":"Name","value":"finishedAfter"}}},{"kind":"Argument","name":{"kind":"Name","value":"cursor"},"value":{"kind":"Variable","name":{"kind":"Name","value":"cursor"}}},{"kind":"Argument","name":{"kind":"Name","value":"limit"},"value":{"kind":"Variable","name":{"kind":"Name","value":"limit"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"trigger"}},{"kind":"Field","name":{"kind":"Name","value":"image"}},{"kind":"Field","name":{"kind":"Name","value":"rollbackOf"}},{"kind":"Field","name":{"kind":"Name","value":"commitId"}},{"kind":"Field","name":{"kind":"Name","value":"commitMessage"}},{"kind":"Field","name":{"kind":"Name","value":"commitCreatedAt"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"updatedAt"}},{"kind":"Field","name":{"kind":"Name","value":"startedAt"}},{"kind":"Field","name":{"kind":"Name","value":"finishedAt"}},{"kind":"Field","name":{"kind":"Name","value":"preDeployStatus"}}]}}]}}]} as unknown as DocumentNode<DeploysQuery, DeploysQueryVariables>;
export const EnvGroupsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"EnvGroups"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"envGroups"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"ownerId"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"updatedAt"}},{"kind":"Field","name":{"kind":"Name","value":"serviceLinks"}},{"kind":"Field","name":{"kind":"Name","value":"envVars"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"key"}}]}},{"kind":"Field","name":{"kind":"Name","value":"secretFiles"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}}]}}]}}]}}]} as unknown as DocumentNode<EnvGroupsQuery, EnvGroupsQueryVariables>;
export const EnvGroupDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"EnvGroup"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"envGroup"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"ownerId"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"updatedAt"}},{"kind":"Field","name":{"kind":"Name","value":"serviceLinks"}},{"kind":"Field","name":{"kind":"Name","value":"envVars"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"key"}}]}},{"kind":"Field","name":{"kind":"Name","value":"secretFiles"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}}]}}]}}]}}]} as unknown as DocumentNode<EnvGroupQuery, EnvGroupQueryVariables>;
export const EnvGroupVarValueDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"EnvGroupVarValue"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"key"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"envGroupVar"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"key"},"value":{"kind":"Variable","name":{"kind":"Name","value":"key"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"key"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}}]}}]} as unknown as DocumentNode<EnvGroupVarValueQuery, EnvGroupVarValueQueryVariables>;
export const EnvGroupSecretFileContentDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"EnvGroupSecretFileContent"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"envGroupSecretFile"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"content"}}]}}]}}]} as unknown as DocumentNode<EnvGroupSecretFileContentQuery, EnvGroupSecretFileContentQueryVariables>;
export const CreateEnvGroupDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateEnvGroup"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"envVars"}},"type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"EnvGroupVarInput"}}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"secretFiles"}},"type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"EnvGroupSecretFileInput"}}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceIds"}},"type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createEnvGroup"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}},{"kind":"Argument","name":{"kind":"Name","value":"envVars"},"value":{"kind":"Variable","name":{"kind":"Name","value":"envVars"}}},{"kind":"Argument","name":{"kind":"Name","value":"secretFiles"},"value":{"kind":"Variable","name":{"kind":"Name","value":"secretFiles"}}},{"kind":"Argument","name":{"kind":"Name","value":"serviceIds"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceIds"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}}]}}]}}]} as unknown as DocumentNode<CreateEnvGroupMutation, CreateEnvGroupMutationVariables>;
export const RenameEnvGroupDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RenameEnvGroup"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"renameEnvGroup"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}}]}}]}}]} as unknown as DocumentNode<RenameEnvGroupMutation, RenameEnvGroupMutationVariables>;
export const DeleteEnvGroupDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteEnvGroup"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteEnvGroup"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<DeleteEnvGroupMutation, DeleteEnvGroupMutationVariables>;
export const SetEnvGroupVarsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetEnvGroupVars"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"envVars"}},"type":{"kind":"NonNullType","type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"EnvGroupVarInput"}}}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setEnvGroupVars"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"envVars"},"value":{"kind":"Variable","name":{"kind":"Name","value":"envVars"}}}]}]}}]} as unknown as DocumentNode<SetEnvGroupVarsMutation, SetEnvGroupVarsMutationVariables>;
export const SetEnvGroupVarDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetEnvGroupVar"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"key"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"value"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setEnvGroupVar"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"key"},"value":{"kind":"Variable","name":{"kind":"Name","value":"key"}}},{"kind":"Argument","name":{"kind":"Name","value":"value"},"value":{"kind":"Variable","name":{"kind":"Name","value":"value"}}}]}]}}]} as unknown as DocumentNode<SetEnvGroupVarMutation, SetEnvGroupVarMutationVariables>;
export const DeleteEnvGroupVarDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteEnvGroupVar"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"key"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteEnvGroupVar"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"key"},"value":{"kind":"Variable","name":{"kind":"Name","value":"key"}}}]}]}}]} as unknown as DocumentNode<DeleteEnvGroupVarMutation, DeleteEnvGroupVarMutationVariables>;
export const SetEnvGroupSecretFileDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetEnvGroupSecretFile"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"content"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setEnvGroupSecretFile"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"content"},"value":{"kind":"Variable","name":{"kind":"Name","value":"content"}}}]}]}}]} as unknown as DocumentNode<SetEnvGroupSecretFileMutation, SetEnvGroupSecretFileMutationVariables>;
export const DeleteEnvGroupSecretFileDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteEnvGroupSecretFile"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteEnvGroupSecretFile"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}]}]}}]} as unknown as DocumentNode<DeleteEnvGroupSecretFileMutation, DeleteEnvGroupSecretFileMutationVariables>;
export const LinkEnvGroupDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"LinkEnvGroup"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"linkEnvGroup"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}}]}]}}]} as unknown as DocumentNode<LinkEnvGroupMutation, LinkEnvGroupMutationVariables>;
export const UnlinkEnvGroupDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"UnlinkEnvGroup"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"unlinkEnvGroup"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}}]}]}}]} as unknown as DocumentNode<UnlinkEnvGroupMutation, UnlinkEnvGroupMutationVariables>;
export const EnvironmentsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Environments"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"projectId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"environments"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"projectId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"projectId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"EnvironmentFields"}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"EnvironmentFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Environment"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"projectId"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"ownerId"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"serviceIds"}},{"kind":"Field","name":{"kind":"Name","value":"databaseIds"}},{"kind":"Field","name":{"kind":"Name","value":"keyValueIds"}},{"kind":"Field","name":{"kind":"Name","value":"envGroupIds"}},{"kind":"Field","name":{"kind":"Name","value":"protectedStatus"}},{"kind":"Field","name":{"kind":"Name","value":"networkIsolationEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowList"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowListEntries"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"cidrBlock"}},{"kind":"Field","name":{"kind":"Name","value":"description"}}]}}]}}]} as unknown as DocumentNode<EnvironmentsQuery, EnvironmentsQueryVariables>;
export const CreateEnvironmentDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateEnvironment"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"projectId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createEnvironment"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"projectId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"projectId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"EnvironmentFields"}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"EnvironmentFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Environment"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"projectId"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"ownerId"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"serviceIds"}},{"kind":"Field","name":{"kind":"Name","value":"databaseIds"}},{"kind":"Field","name":{"kind":"Name","value":"keyValueIds"}},{"kind":"Field","name":{"kind":"Name","value":"envGroupIds"}},{"kind":"Field","name":{"kind":"Name","value":"protectedStatus"}},{"kind":"Field","name":{"kind":"Name","value":"networkIsolationEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowList"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowListEntries"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"cidrBlock"}},{"kind":"Field","name":{"kind":"Name","value":"description"}}]}}]}}]} as unknown as DocumentNode<CreateEnvironmentMutation, CreateEnvironmentMutationVariables>;
export const RenameEnvironmentDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RenameEnvironment"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"renameEnvironment"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"EnvironmentFields"}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"EnvironmentFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Environment"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"projectId"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"ownerId"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"serviceIds"}},{"kind":"Field","name":{"kind":"Name","value":"databaseIds"}},{"kind":"Field","name":{"kind":"Name","value":"keyValueIds"}},{"kind":"Field","name":{"kind":"Name","value":"envGroupIds"}},{"kind":"Field","name":{"kind":"Name","value":"protectedStatus"}},{"kind":"Field","name":{"kind":"Name","value":"networkIsolationEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowList"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowListEntries"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"cidrBlock"}},{"kind":"Field","name":{"kind":"Name","value":"description"}}]}}]}}]} as unknown as DocumentNode<RenameEnvironmentMutation, RenameEnvironmentMutationVariables>;
export const DeleteEnvironmentDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteEnvironment"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteEnvironment"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<DeleteEnvironmentMutation, DeleteEnvironmentMutationVariables>;
export const SetEnvironmentServicesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetEnvironmentServices"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceIds"}},"type":{"kind":"NonNullType","type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setEnvironmentServices"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"serviceIds"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceIds"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"EnvironmentFields"}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"EnvironmentFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Environment"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"projectId"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"ownerId"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"serviceIds"}},{"kind":"Field","name":{"kind":"Name","value":"databaseIds"}},{"kind":"Field","name":{"kind":"Name","value":"keyValueIds"}},{"kind":"Field","name":{"kind":"Name","value":"envGroupIds"}},{"kind":"Field","name":{"kind":"Name","value":"protectedStatus"}},{"kind":"Field","name":{"kind":"Name","value":"networkIsolationEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowList"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowListEntries"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"cidrBlock"}},{"kind":"Field","name":{"kind":"Name","value":"description"}}]}}]}}]} as unknown as DocumentNode<SetEnvironmentServicesMutation, SetEnvironmentServicesMutationVariables>;
export const SetEnvironmentDatabasesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetEnvironmentDatabases"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"databaseIds"}},"type":{"kind":"NonNullType","type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setEnvironmentDatabases"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"databaseIds"},"value":{"kind":"Variable","name":{"kind":"Name","value":"databaseIds"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"EnvironmentFields"}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"EnvironmentFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Environment"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"projectId"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"ownerId"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"serviceIds"}},{"kind":"Field","name":{"kind":"Name","value":"databaseIds"}},{"kind":"Field","name":{"kind":"Name","value":"keyValueIds"}},{"kind":"Field","name":{"kind":"Name","value":"envGroupIds"}},{"kind":"Field","name":{"kind":"Name","value":"protectedStatus"}},{"kind":"Field","name":{"kind":"Name","value":"networkIsolationEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowList"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowListEntries"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"cidrBlock"}},{"kind":"Field","name":{"kind":"Name","value":"description"}}]}}]}}]} as unknown as DocumentNode<SetEnvironmentDatabasesMutation, SetEnvironmentDatabasesMutationVariables>;
export const SetEnvironmentKeyValuesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetEnvironmentKeyValues"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"keyValueIds"}},"type":{"kind":"NonNullType","type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setEnvironmentKeyValues"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"keyValueIds"},"value":{"kind":"Variable","name":{"kind":"Name","value":"keyValueIds"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"EnvironmentFields"}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"EnvironmentFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Environment"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"projectId"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"ownerId"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"serviceIds"}},{"kind":"Field","name":{"kind":"Name","value":"databaseIds"}},{"kind":"Field","name":{"kind":"Name","value":"keyValueIds"}},{"kind":"Field","name":{"kind":"Name","value":"envGroupIds"}},{"kind":"Field","name":{"kind":"Name","value":"protectedStatus"}},{"kind":"Field","name":{"kind":"Name","value":"networkIsolationEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowList"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowListEntries"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"cidrBlock"}},{"kind":"Field","name":{"kind":"Name","value":"description"}}]}}]}}]} as unknown as DocumentNode<SetEnvironmentKeyValuesMutation, SetEnvironmentKeyValuesMutationVariables>;
export const SetEnvironmentEnvGroupsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetEnvironmentEnvGroups"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"envGroupIds"}},"type":{"kind":"NonNullType","type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setEnvironmentEnvGroups"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"envGroupIds"},"value":{"kind":"Variable","name":{"kind":"Name","value":"envGroupIds"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"EnvironmentFields"}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"EnvironmentFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Environment"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"projectId"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"ownerId"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"serviceIds"}},{"kind":"Field","name":{"kind":"Name","value":"databaseIds"}},{"kind":"Field","name":{"kind":"Name","value":"keyValueIds"}},{"kind":"Field","name":{"kind":"Name","value":"envGroupIds"}},{"kind":"Field","name":{"kind":"Name","value":"protectedStatus"}},{"kind":"Field","name":{"kind":"Name","value":"networkIsolationEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowList"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowListEntries"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"cidrBlock"}},{"kind":"Field","name":{"kind":"Name","value":"description"}}]}}]}}]} as unknown as DocumentNode<SetEnvironmentEnvGroupsMutation, SetEnvironmentEnvGroupsMutationVariables>;
export const SetEnvironmentAclDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetEnvironmentACL"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"protectedStatus"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"networkIsolationEnabled"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ipAllowListEntries"}},"type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"IPAllowListEntryInput"}}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setEnvironmentACL"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"protectedStatus"},"value":{"kind":"Variable","name":{"kind":"Name","value":"protectedStatus"}}},{"kind":"Argument","name":{"kind":"Name","value":"networkIsolationEnabled"},"value":{"kind":"Variable","name":{"kind":"Name","value":"networkIsolationEnabled"}}},{"kind":"Argument","name":{"kind":"Name","value":"ipAllowListEntries"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ipAllowListEntries"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"EnvironmentFields"}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"EnvironmentFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Environment"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"projectId"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"ownerId"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"serviceIds"}},{"kind":"Field","name":{"kind":"Name","value":"databaseIds"}},{"kind":"Field","name":{"kind":"Name","value":"keyValueIds"}},{"kind":"Field","name":{"kind":"Name","value":"envGroupIds"}},{"kind":"Field","name":{"kind":"Name","value":"protectedStatus"}},{"kind":"Field","name":{"kind":"Name","value":"networkIsolationEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowList"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowListEntries"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"cidrBlock"}},{"kind":"Field","name":{"kind":"Name","value":"description"}}]}}]}}]} as unknown as DocumentNode<SetEnvironmentAclMutation, SetEnvironmentAclMutationVariables>;
export const ReposDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Repos"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"repos"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"fullName"}},{"kind":"Field","name":{"kind":"Name","value":"private"}},{"kind":"Field","name":{"kind":"Name","value":"defaultBranch"}},{"kind":"Field","name":{"kind":"Name","value":"htmlUrl"}},{"kind":"Field","name":{"kind":"Name","value":"cloneUrl"}}]}}]}}]} as unknown as DocumentNode<ReposQuery, ReposQueryVariables>;
export const GitConnectionDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GitConnection"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"gitConnection"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"connected"}},{"kind":"Field","name":{"kind":"Name","value":"accountLogin"}},{"kind":"Field","name":{"kind":"Name","value":"installUrl"}}]}}]}}]} as unknown as DocumentNode<GitConnectionQuery, GitConnectionQueryVariables>;
export const ConnectGitDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ConnectGit"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"connectGit"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"connected"}},{"kind":"Field","name":{"kind":"Name","value":"installUrl"}}]}}]}}]} as unknown as DocumentNode<ConnectGitMutation, ConnectGitMutationVariables>;
export const DisconnectGitDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DisconnectGit"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"disconnectGit"}}]}}]} as unknown as DocumentNode<DisconnectGitMutation, DisconnectGitMutationVariables>;
export const KeyValuesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"KeyValues"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"keyValues"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"version"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"externalHost"}},{"kind":"Field","name":{"kind":"Name","value":"public"}}]}}]}}]} as unknown as DocumentNode<KeyValuesQuery, KeyValuesQueryVariables>;
export const KeyValueDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"KeyValue"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"keyValue"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"version"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"externalHost"}},{"kind":"Field","name":{"kind":"Name","value":"public"}}]}}]}}]} as unknown as DocumentNode<KeyValueQuery, KeyValueQueryVariables>;
export const KeyValueConnectionInfoDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"KeyValueConnectionInfo"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"keyValueConnectionInfo"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"internalConnectionString"}},{"kind":"Field","name":{"kind":"Name","value":"externalConnectionString"}},{"kind":"Field","name":{"kind":"Name","value":"cliCommand"}}]}}]}}]} as unknown as DocumentNode<KeyValueConnectionInfoQuery, KeyValueConnectionInfoQueryVariables>;
export const KeyValueInstanceTypesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"KeyValueInstanceTypes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"keyValueInstanceTypes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"cpu"}},{"kind":"Field","name":{"kind":"Name","value":"memory"}},{"kind":"Field","name":{"kind":"Name","value":"storageGB"}}]}}]}}]} as unknown as DocumentNode<KeyValueInstanceTypesQuery, KeyValueInstanceTypesQueryVariables>;
export const KeyValueIpAllowListDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"KeyValueIpAllowList"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"keyValue"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowListEntries"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"cidrBlock"}},{"kind":"Field","name":{"kind":"Name","value":"description"}}]}}]}}]}}]} as unknown as DocumentNode<KeyValueIpAllowListQuery, KeyValueIpAllowListQueryVariables>;
export const CreateKeyValueDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateKeyValue"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"environmentId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"plan"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"version"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"storageGB"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"public"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"maxmemoryPolicy"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"persistenceMode"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createKeyValue"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}},{"kind":"Argument","name":{"kind":"Name","value":"environmentId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"environmentId"}}},{"kind":"Argument","name":{"kind":"Name","value":"plan"},"value":{"kind":"Variable","name":{"kind":"Name","value":"plan"}}},{"kind":"Argument","name":{"kind":"Name","value":"version"},"value":{"kind":"Variable","name":{"kind":"Name","value":"version"}}},{"kind":"Argument","name":{"kind":"Name","value":"storageGB"},"value":{"kind":"Variable","name":{"kind":"Name","value":"storageGB"}}},{"kind":"Argument","name":{"kind":"Name","value":"public"},"value":{"kind":"Variable","name":{"kind":"Name","value":"public"}}},{"kind":"Argument","name":{"kind":"Name","value":"maxmemoryPolicy"},"value":{"kind":"Variable","name":{"kind":"Name","value":"maxmemoryPolicy"}}},{"kind":"Argument","name":{"kind":"Name","value":"persistenceMode"},"value":{"kind":"Variable","name":{"kind":"Name","value":"persistenceMode"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"projectId"}},{"kind":"Field","name":{"kind":"Name","value":"environmentId"}}]}}]}}]} as unknown as DocumentNode<CreateKeyValueMutation, CreateKeyValueMutationVariables>;
export const SetKeyValueIpAllowListDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetKeyValueIpAllowList"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"entries"}},"type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"IPAllowListEntryInput"}}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setKeyValueIpAllowList"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"entries"},"value":{"kind":"Variable","name":{"kind":"Name","value":"entries"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowListEntries"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"cidrBlock"}},{"kind":"Field","name":{"kind":"Name","value":"description"}}]}}]}}]}}]} as unknown as DocumentNode<SetKeyValueIpAllowListMutation, SetKeyValueIpAllowListMutationVariables>;
export const UpdateKeyValuePlanDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"UpdateKeyValuePlan"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"plan"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"updateKeyValuePlan"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"plan"},"value":{"kind":"Variable","name":{"kind":"Name","value":"plan"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"status"}}]}}]}}]} as unknown as DocumentNode<UpdateKeyValuePlanMutation, UpdateKeyValuePlanMutationVariables>;
export const RenameKeyValueDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RenameKeyValue"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"renameKeyValue"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}}]}}]}}]} as unknown as DocumentNode<RenameKeyValueMutation, RenameKeyValueMutationVariables>;
export const DeleteKeyValueDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteKeyValue"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteKeyValue"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<DeleteKeyValueMutation, DeleteKeyValueMutationVariables>;
export const SuspendKeyValueDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SuspendKeyValue"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"suspendKeyValue"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}}]}}]}}]} as unknown as DocumentNode<SuspendKeyValueMutation, SuspendKeyValueMutationVariables>;
export const ResumeKeyValueDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ResumeKeyValue"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"resumeKeyValue"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}}]}}]}}]} as unknown as DocumentNode<ResumeKeyValueMutation, ResumeKeyValueMutationVariables>;
export const LogsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Logs"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"resource"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"type"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"text"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"level"}},"type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"instance"}},"type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"statusCode"}},"type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"method"}},"type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"path"}},"type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"startTime"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"endTime"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"limit"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"logs"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"resource"},"value":{"kind":"Variable","name":{"kind":"Name","value":"resource"}}},{"kind":"Argument","name":{"kind":"Name","value":"type"},"value":{"kind":"Variable","name":{"kind":"Name","value":"type"}}},{"kind":"Argument","name":{"kind":"Name","value":"text"},"value":{"kind":"Variable","name":{"kind":"Name","value":"text"}}},{"kind":"Argument","name":{"kind":"Name","value":"level"},"value":{"kind":"Variable","name":{"kind":"Name","value":"level"}}},{"kind":"Argument","name":{"kind":"Name","value":"instance"},"value":{"kind":"Variable","name":{"kind":"Name","value":"instance"}}},{"kind":"Argument","name":{"kind":"Name","value":"statusCode"},"value":{"kind":"Variable","name":{"kind":"Name","value":"statusCode"}}},{"kind":"Argument","name":{"kind":"Name","value":"method"},"value":{"kind":"Variable","name":{"kind":"Name","value":"method"}}},{"kind":"Argument","name":{"kind":"Name","value":"path"},"value":{"kind":"Variable","name":{"kind":"Name","value":"path"}}},{"kind":"Argument","name":{"kind":"Name","value":"startTime"},"value":{"kind":"Variable","name":{"kind":"Name","value":"startTime"}}},{"kind":"Argument","name":{"kind":"Name","value":"endTime"},"value":{"kind":"Variable","name":{"kind":"Name","value":"endTime"}}},{"kind":"Argument","name":{"kind":"Name","value":"limit"},"value":{"kind":"Variable","name":{"kind":"Name","value":"limit"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"timestamp"}},{"kind":"Field","name":{"kind":"Name","value":"message"}},{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"instance"}},{"kind":"Field","name":{"kind":"Name","value":"level"}},{"kind":"Field","name":{"kind":"Name","value":"method"}},{"kind":"Field","name":{"kind":"Name","value":"statusCode"}}]}}]}}]} as unknown as DocumentNode<LogsQuery, LogsQueryVariables>;
export const LogLabelValuesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"LogLabelValues"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"label"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"resource"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"logLabelValues"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"label"},"value":{"kind":"Variable","name":{"kind":"Name","value":"label"}}},{"kind":"Argument","name":{"kind":"Name","value":"resource"},"value":{"kind":"Variable","name":{"kind":"Name","value":"resource"}}}]}]}}]} as unknown as DocumentNode<LogLabelValuesQuery, LogLabelValuesQueryVariables>;
export const MetricsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Metrics"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"query"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"MetricsQueryInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"metrics"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"query"},"value":{"kind":"Variable","name":{"kind":"Name","value":"query"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"unit"}},{"kind":"Field","name":{"kind":"Name","value":"labels"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"field"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}},{"kind":"Field","name":{"kind":"Name","value":"values"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"time"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}},{"kind":"Field","name":{"kind":"Name","value":"parameters"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"quantile"}}]}}]}}]}}]} as unknown as DocumentNode<MetricsQuery, MetricsQueryVariables>;
export const MonthToDateBandwidthDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"MonthToDateBandwidth"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"resourceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"monthToDateBandwidth"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"resourceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"resourceId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"egressBandwidthMB"}},{"kind":"Field","name":{"kind":"Name","value":"httpEgressBandwidthMB"}},{"kind":"Field","name":{"kind":"Name","value":"natEgressBandwidthMB"}},{"kind":"Field","name":{"kind":"Name","value":"privateLinkEgressBandwidthMB"}},{"kind":"Field","name":{"kind":"Name","value":"websocketEgressBandwidthMB"}}]}}]}}]} as unknown as DocumentNode<MonthToDateBandwidthQuery, MonthToDateBandwidthQueryVariables>;
export const MetricsFiltersDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"MetricsFilters"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"query"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"MetricsFiltersQueryInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"metricsFilters"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"query"},"value":{"kind":"Variable","name":{"kind":"Name","value":"query"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"values"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"field"}},{"kind":"Field","name":{"kind":"Name","value":"values"}}]}}]}}]}}]} as unknown as DocumentNode<MetricsFiltersQuery, MetricsFiltersQueryVariables>;
export const MetricsPathFilterSuggestionsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"MetricsPathFilterSuggestions"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"query"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"MetricsPathFilterSuggestionsInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"metricsPathFilterSuggestions"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"query"},"value":{"kind":"Variable","name":{"kind":"Name","value":"query"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"paths"}}]}}]}}]} as unknown as DocumentNode<MetricsPathFilterSuggestionsQuery, MetricsPathFilterSuggestionsQueryVariables>;
export const DatastoreMetricsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatastoreMetrics"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"query"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"DatastoreMetricsQueryInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"datastoreMetrics"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"query"},"value":{"kind":"Variable","name":{"kind":"Name","value":"query"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"unit"}},{"kind":"Field","name":{"kind":"Name","value":"labels"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"field"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}},{"kind":"Field","name":{"kind":"Name","value":"values"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"time"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}}]}}]}}]} as unknown as DocumentNode<DatastoreMetricsQuery, DatastoreMetricsQueryVariables>;
export const NotificationSettingsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"NotificationSettings"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"notificationSettings"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deployStarted"}},{"kind":"Field","name":{"kind":"Name","value":"deploySucceeded"}},{"kind":"Field","name":{"kind":"Name","value":"deployFailed"}}]}}]}}]} as unknown as DocumentNode<NotificationSettingsQuery, NotificationSettingsQueryVariables>;
export const UpdateNotificationSettingsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"UpdateNotificationSettings"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"deployStarted"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"deploySucceeded"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"deployFailed"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"updateNotificationSettings"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"deployStarted"},"value":{"kind":"Variable","name":{"kind":"Name","value":"deployStarted"}}},{"kind":"Argument","name":{"kind":"Name","value":"deploySucceeded"},"value":{"kind":"Variable","name":{"kind":"Name","value":"deploySucceeded"}}},{"kind":"Argument","name":{"kind":"Name","value":"deployFailed"},"value":{"kind":"Variable","name":{"kind":"Name","value":"deployFailed"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deployStarted"}},{"kind":"Field","name":{"kind":"Name","value":"deploySucceeded"}},{"kind":"Field","name":{"kind":"Name","value":"deployFailed"}}]}}]}}]} as unknown as DocumentNode<UpdateNotificationSettingsMutation, UpdateNotificationSettingsMutationVariables>;
export const ProjectsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Projects"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"projects"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"ProjectFields"}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"ProjectFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Project"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"ownerId"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"serviceIds"}},{"kind":"Field","name":{"kind":"Name","value":"databaseIds"}},{"kind":"Field","name":{"kind":"Name","value":"keyValueIds"}}]}}]} as unknown as DocumentNode<ProjectsQuery, ProjectsQueryVariables>;
export const CreateProjectDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateProject"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createProject"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"ProjectFields"}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"ProjectFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Project"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"ownerId"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"serviceIds"}},{"kind":"Field","name":{"kind":"Name","value":"databaseIds"}},{"kind":"Field","name":{"kind":"Name","value":"keyValueIds"}}]}}]} as unknown as DocumentNode<CreateProjectMutation, CreateProjectMutationVariables>;
export const RenameProjectDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RenameProject"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"renameProject"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"ProjectFields"}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"ProjectFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Project"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"ownerId"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"serviceIds"}},{"kind":"Field","name":{"kind":"Name","value":"databaseIds"}},{"kind":"Field","name":{"kind":"Name","value":"keyValueIds"}}]}}]} as unknown as DocumentNode<RenameProjectMutation, RenameProjectMutationVariables>;
export const DeleteProjectDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteProject"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteProject"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<DeleteProjectMutation, DeleteProjectMutationVariables>;
export const SetProjectServicesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetProjectServices"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceIds"}},"type":{"kind":"NonNullType","type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setProjectServices"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"serviceIds"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceIds"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"ProjectFields"}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"ProjectFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Project"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"ownerId"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"serviceIds"}},{"kind":"Field","name":{"kind":"Name","value":"databaseIds"}},{"kind":"Field","name":{"kind":"Name","value":"keyValueIds"}}]}}]} as unknown as DocumentNode<SetProjectServicesMutation, SetProjectServicesMutationVariables>;
export const SetProjectDatabasesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetProjectDatabases"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"databaseIds"}},"type":{"kind":"NonNullType","type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setProjectDatabases"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"databaseIds"},"value":{"kind":"Variable","name":{"kind":"Name","value":"databaseIds"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"ProjectFields"}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"ProjectFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Project"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"ownerId"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"serviceIds"}},{"kind":"Field","name":{"kind":"Name","value":"databaseIds"}},{"kind":"Field","name":{"kind":"Name","value":"keyValueIds"}}]}}]} as unknown as DocumentNode<SetProjectDatabasesMutation, SetProjectDatabasesMutationVariables>;
export const SetProjectKeyValuesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetProjectKeyValues"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"keyValueIds"}},"type":{"kind":"NonNullType","type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setProjectKeyValues"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"keyValueIds"},"value":{"kind":"Variable","name":{"kind":"Name","value":"keyValueIds"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"ProjectFields"}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"ProjectFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Project"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"ownerId"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"serviceIds"}},{"kind":"Field","name":{"kind":"Name","value":"databaseIds"}},{"kind":"Field","name":{"kind":"Name","value":"keyValueIds"}}]}}]} as unknown as DocumentNode<SetProjectKeyValuesMutation, SetProjectKeyValuesMutationVariables>;
export const RegistryCredentialsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"RegistryCredentials"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"registryCredentials"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"host"}},{"kind":"Field","name":{"kind":"Name","value":"username"}},{"kind":"Field","name":{"kind":"Name","value":"expiresAt"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<RegistryCredentialsQuery, RegistryCredentialsQueryVariables>;
export const CreateRegistryCredentialDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateRegistryCredential"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"host"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"username"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"authToken"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"expiresAt"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createRegistryCredential"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"host"},"value":{"kind":"Variable","name":{"kind":"Name","value":"host"}}},{"kind":"Argument","name":{"kind":"Name","value":"username"},"value":{"kind":"Variable","name":{"kind":"Name","value":"username"}}},{"kind":"Argument","name":{"kind":"Name","value":"authToken"},"value":{"kind":"Variable","name":{"kind":"Name","value":"authToken"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"expiresAt"},"value":{"kind":"Variable","name":{"kind":"Name","value":"expiresAt"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"host"}},{"kind":"Field","name":{"kind":"Name","value":"username"}},{"kind":"Field","name":{"kind":"Name","value":"expiresAt"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<CreateRegistryCredentialMutation, CreateRegistryCredentialMutationVariables>;
export const DeleteRegistryCredentialDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteRegistryCredential"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteRegistryCredential"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<DeleteRegistryCredentialMutation, DeleteRegistryCredentialMutationVariables>;
export const SetAutoDeployDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetAutoDeploy"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"enabled"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setAutoDeploy"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"enabled"},"value":{"kind":"Variable","name":{"kind":"Name","value":"enabled"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"autoDeploy"}}]}}]}}]} as unknown as DocumentNode<SetAutoDeployMutation, SetAutoDeployMutationVariables>;
export const AutoscalingConfigDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"AutoscalingConfig"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"autoscalingConfig"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"enabled"}},{"kind":"Field","name":{"kind":"Name","value":"minInstances"}},{"kind":"Field","name":{"kind":"Name","value":"maxInstances"}},{"kind":"Field","name":{"kind":"Name","value":"targetCPUPercent"}},{"kind":"Field","name":{"kind":"Name","value":"targetMemoryPercent"}}]}}]}}]} as unknown as DocumentNode<AutoscalingConfigQuery, AutoscalingConfigQueryVariables>;
export const SetAutoscalingDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetAutoscaling"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"minInstances"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"maxInstances"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"targetCPUPercent"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"targetMemoryPercent"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setAutoscaling"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"minInstances"},"value":{"kind":"Variable","name":{"kind":"Name","value":"minInstances"}}},{"kind":"Argument","name":{"kind":"Name","value":"maxInstances"},"value":{"kind":"Variable","name":{"kind":"Name","value":"maxInstances"}}},{"kind":"Argument","name":{"kind":"Name","value":"targetCPUPercent"},"value":{"kind":"Variable","name":{"kind":"Name","value":"targetCPUPercent"}}},{"kind":"Argument","name":{"kind":"Name","value":"targetMemoryPercent"},"value":{"kind":"Variable","name":{"kind":"Name","value":"targetMemoryPercent"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"enabled"}},{"kind":"Field","name":{"kind":"Name","value":"minInstances"}},{"kind":"Field","name":{"kind":"Name","value":"maxInstances"}},{"kind":"Field","name":{"kind":"Name","value":"targetCPUPercent"}},{"kind":"Field","name":{"kind":"Name","value":"targetMemoryPercent"}}]}}]}}]} as unknown as DocumentNode<SetAutoscalingMutation, SetAutoscalingMutationVariables>;
export const DisableAutoscalingDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DisableAutoscaling"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"disableAutoscaling"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<DisableAutoscalingMutation, DisableAutoscalingMutationVariables>;
export const SetBuildFilterDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetBuildFilter"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"buildFilter"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"BuildFilterInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setBuildFilter"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"buildFilter"},"value":{"kind":"Variable","name":{"kind":"Name","value":"buildFilter"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"buildFilter"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"paths"}},{"kind":"Field","name":{"kind":"Name","value":"ignoredPaths"}}]}},{"kind":"Field","name":{"kind":"Name","value":"phase"}}]}}]}}]} as unknown as DocumentNode<SetBuildFilterMutation, SetBuildFilterMutationVariables>;
export const SetBuildCommandDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetBuildCommand"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"command"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setBuildCommand"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"command"},"value":{"kind":"Variable","name":{"kind":"Name","value":"command"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"buildCommand"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}}]}}]}}]} as unknown as DocumentNode<SetBuildCommandMutation, SetBuildCommandMutationVariables>;
export const SetStartCommandDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetStartCommand"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"command"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setStartCommand"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"command"},"value":{"kind":"Variable","name":{"kind":"Name","value":"command"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"startCommand"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}}]}}]}}]} as unknown as DocumentNode<SetStartCommandMutation, SetStartCommandMutationVariables>;
export const SetDockerfilePathDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetDockerfilePath"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"dockerfilePath"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setDockerfilePath"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"dockerfilePath"},"value":{"kind":"Variable","name":{"kind":"Name","value":"dockerfilePath"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"dockerfilePath"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}}]}}]}}]} as unknown as DocumentNode<SetDockerfilePathMutation, SetDockerfilePathMutationVariables>;
export const CronJobRunsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"CronJobRuns"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"cursor"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"limit"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"cronJobRuns"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"cursor"},"value":{"kind":"Variable","name":{"kind":"Name","value":"cursor"}}},{"kind":"Argument","name":{"kind":"Name","value":"limit"},"value":{"kind":"Variable","name":{"kind":"Name","value":"limit"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"startedAt"}},{"kind":"Field","name":{"kind":"Name","value":"finishedAt"}}]}}]}}]} as unknown as DocumentNode<CronJobRunsQuery, CronJobRunsQueryVariables>;
export const CancelCronJobRunDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CancelCronJobRun"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"runId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"cancelCronJobRun"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"runId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"runId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"startedAt"}},{"kind":"Field","name":{"kind":"Name","value":"finishedAt"}}]}}]}}]} as unknown as DocumentNode<CancelCronJobRunMutation, CancelCronJobRunMutationVariables>;
export const CustomDomainsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"CustomDomains"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"customDomains"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"domainType"}},{"kind":"Field","name":{"kind":"Name","value":"verificationStatus"}},{"kind":"Field","name":{"kind":"Name","value":"serverStatus"}},{"kind":"Field","name":{"kind":"Name","value":"redirectForName"}},{"kind":"Field","name":{"kind":"Name","value":"dnsRecord"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}}]}}]}}]} as unknown as DocumentNode<CustomDomainsQuery, CustomDomainsQueryVariables>;
export const AddCustomDomainDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"AddCustomDomain"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"addCustomDomain"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"domainType"}},{"kind":"Field","name":{"kind":"Name","value":"verificationStatus"}},{"kind":"Field","name":{"kind":"Name","value":"serverStatus"}},{"kind":"Field","name":{"kind":"Name","value":"redirectForName"}},{"kind":"Field","name":{"kind":"Name","value":"dnsRecord"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}}]}}]}}]} as unknown as DocumentNode<AddCustomDomainMutation, AddCustomDomainMutationVariables>;
export const DeleteCustomDomainDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteCustomDomain"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteCustomDomain"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}]}]}}]} as unknown as DocumentNode<DeleteCustomDomainMutation, DeleteCustomDomainMutationVariables>;
export const VerifyCustomDomainDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"VerifyCustomDomain"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"verifyCustomDomain"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"domainType"}},{"kind":"Field","name":{"kind":"Name","value":"verificationStatus"}},{"kind":"Field","name":{"kind":"Name","value":"serverStatus"}},{"kind":"Field","name":{"kind":"Name","value":"redirectForName"}},{"kind":"Field","name":{"kind":"Name","value":"dnsRecord"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}}]}}]}}]} as unknown as DocumentNode<VerifyCustomDomainMutation, VerifyCustomDomainMutationVariables>;
export const DeployHookDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DeployHook"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deployHook"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"url"}}]}}]}}]} as unknown as DocumentNode<DeployHookQuery, DeployHookQueryVariables>;
export const RegenerateDeployHookDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RegenerateDeployHook"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"regenerateDeployHook"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"url"}}]}}]}}]} as unknown as DocumentNode<RegenerateDeployHookMutation, RegenerateDeployHookMutationVariables>;
export const SetDisplayNameDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetDisplayName"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"displayName"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setDisplayName"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"displayName"},"value":{"kind":"Variable","name":{"kind":"Name","value":"displayName"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"displayName"}}]}}]}}]} as unknown as DocumentNode<SetDisplayNameMutation, SetDisplayNameMutationVariables>;
export const EnvVarKeysDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"EnvVarKeys"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"cursor"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"limit"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"envVars"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"cursor"},"value":{"kind":"Variable","name":{"kind":"Name","value":"cursor"}}},{"kind":"Argument","name":{"kind":"Name","value":"limit"},"value":{"kind":"Variable","name":{"kind":"Name","value":"limit"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"envVar"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"key"}}]}},{"kind":"Field","name":{"kind":"Name","value":"cursor"}}]}}]}}]} as unknown as DocumentNode<EnvVarKeysQuery, EnvVarKeysQueryVariables>;
export const EnvVarValueDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"EnvVarValue"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"key"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"service"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"envVar"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"key"},"value":{"kind":"Variable","name":{"kind":"Name","value":"key"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"key"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}}]}}]}}]} as unknown as DocumentNode<EnvVarValueQuery, EnvVarValueQueryVariables>;
export const SetEnvVarsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetEnvVars"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"envVars"}},"type":{"kind":"NonNullType","type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"EnvVarInput"}}}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setEnvVars"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"envVars"},"value":{"kind":"Variable","name":{"kind":"Name","value":"envVars"}}}]}]}}]} as unknown as DocumentNode<SetEnvVarsMutation, SetEnvVarsMutationVariables>;
export const SetEnvVarDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetEnvVar"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"key"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"value"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"generateValue"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setEnvVar"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"key"},"value":{"kind":"Variable","name":{"kind":"Name","value":"key"}}},{"kind":"Argument","name":{"kind":"Name","value":"value"},"value":{"kind":"Variable","name":{"kind":"Name","value":"value"}}},{"kind":"Argument","name":{"kind":"Name","value":"generateValue"},"value":{"kind":"Variable","name":{"kind":"Name","value":"generateValue"}}}]}]}}]} as unknown as DocumentNode<SetEnvVarMutation, SetEnvVarMutationVariables>;
export const DeleteEnvVarDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteEnvVar"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"key"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteEnvVar"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"key"},"value":{"kind":"Variable","name":{"kind":"Name","value":"key"}}}]}]}}]} as unknown as DocumentNode<DeleteEnvVarMutation, DeleteEnvVarMutationVariables>;
export const ServiceEventsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"ServiceEvents"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"cursor"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"limit"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"serviceEvents"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"cursor"},"value":{"kind":"Variable","name":{"kind":"Name","value":"cursor"}}},{"kind":"Argument","name":{"kind":"Name","value":"limit"},"value":{"kind":"Variable","name":{"kind":"Name","value":"limit"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"timestamp"}},{"kind":"Field","name":{"kind":"Name","value":"cursor"}},{"kind":"Field","name":{"kind":"Name","value":"details"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deployId"}},{"kind":"Field","name":{"kind":"Name","value":"deployStatus"}},{"kind":"Field","name":{"kind":"Name","value":"preDeployStatus"}},{"kind":"Field","name":{"kind":"Name","value":"actor"}},{"kind":"Field","name":{"kind":"Name","value":"triggeredByUser"}},{"kind":"Field","name":{"kind":"Name","value":"trigger"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"firstBuild"}},{"kind":"Field","name":{"kind":"Name","value":"envUpdated"}},{"kind":"Field","name":{"kind":"Name","value":"manual"}},{"kind":"Field","name":{"kind":"Name","value":"deployedByRender"}},{"kind":"Field","name":{"kind":"Name","value":"clearCache"}},{"kind":"Field","name":{"kind":"Name","value":"rollback"}}]}}]}}]}}]}}]} as unknown as DocumentNode<ServiceEventsQuery, ServiceEventsQueryVariables>;
export const TriggerDeployDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"TriggerDeploy"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"commitId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"deployMode"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"triggerDeploy"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"commitId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"commitId"}}},{"kind":"Argument","name":{"kind":"Name","value":"deployMode"},"value":{"kind":"Variable","name":{"kind":"Name","value":"deployMode"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"trigger"}},{"kind":"Field","name":{"kind":"Name","value":"rollbackOf"}},{"kind":"Field","name":{"kind":"Name","value":"image"}}]}}]}}]} as unknown as DocumentNode<TriggerDeployMutation, TriggerDeployMutationVariables>;
export const CancelDeployDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CancelDeploy"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"deployId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"cancelDeploy"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"deployId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"deployId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"status"}}]}}]}}]} as unknown as DocumentNode<CancelDeployMutation, CancelDeployMutationVariables>;
export const RollbackServiceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RollbackService"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"deployId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"rollbackService"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"deployId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"deployId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"trigger"}},{"kind":"Field","name":{"kind":"Name","value":"rollbackOf"}},{"kind":"Field","name":{"kind":"Name","value":"image"}}]}}]}}]} as unknown as DocumentNode<RollbackServiceMutation, RollbackServiceMutationVariables>;
export const SetHealthCheckPathDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetHealthCheckPath"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"path"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setHealthCheckPath"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"path"},"value":{"kind":"Variable","name":{"kind":"Name","value":"path"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"healthCheckPath"}}]}}]}}]} as unknown as DocumentNode<SetHealthCheckPathMutation, SetHealthCheckPathMutationVariables>;
export const SetIdleTimeoutDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetIdleTimeout"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"idleTTLSeconds"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setIdleTimeout"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"idleTTLSeconds"},"value":{"kind":"Variable","name":{"kind":"Name","value":"idleTTLSeconds"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"idleTTLSeconds"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}}]}}]}}]} as unknown as DocumentNode<SetIdleTimeoutMutation, SetIdleTimeoutMutationVariables>;
export const InstanceTypesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"InstanceTypes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"instanceTypes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"cpu"}},{"kind":"Field","name":{"kind":"Name","value":"memory"}}]}}]}}]} as unknown as DocumentNode<InstanceTypesQuery, InstanceTypesQueryVariables>;
export const UpdateServicePlanDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"UpdateServicePlan"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"plan"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"updateServicePlan"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"plan"},"value":{"kind":"Variable","name":{"kind":"Name","value":"plan"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}}]}}]}}]} as unknown as DocumentNode<UpdateServicePlanMutation, UpdateServicePlanMutationVariables>;
export const SetServiceIpAllowListDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetServiceIpAllowList"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"cidrs"}},"type":{"kind":"ListType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setServiceIpAllowList"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"cidrs"},"value":{"kind":"Variable","name":{"kind":"Name","value":"cidrs"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowList"}}]}}]}}]} as unknown as DocumentNode<SetServiceIpAllowListMutation, SetServiceIpAllowListMutationVariables>;
export const SetMaintenanceModeDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetMaintenanceMode"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"maintenanceMode"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"MaintenanceModeInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setMaintenanceMode"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"maintenanceMode"},"value":{"kind":"Variable","name":{"kind":"Name","value":"maintenanceMode"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"maintenanceMode"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"enabled"}},{"kind":"Field","name":{"kind":"Name","value":"uri"}}]}}]}}]}}]} as unknown as DocumentNode<SetMaintenanceModeMutation, SetMaintenanceModeMutationVariables>;
export const SetMaxShutdownDelayDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetMaxShutdownDelay"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"seconds"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setMaxShutdownDelay"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"seconds"},"value":{"kind":"Variable","name":{"kind":"Name","value":"seconds"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"maxShutdownDelaySeconds"}}]}}]}}]} as unknown as DocumentNode<SetMaxShutdownDelayMutation, SetMaxShutdownDelayMutationVariables>;
export const SetPreDeployCommandDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetPreDeployCommand"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"command"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setPreDeployCommand"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"command"},"value":{"kind":"Variable","name":{"kind":"Name","value":"command"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"preDeployCommand"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}}]}}]}}]} as unknown as DocumentNode<SetPreDeployCommandMutation, SetPreDeployCommandMutationVariables>;
export const SetRootDirDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetRootDir"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"rootDir"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setRootDir"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"rootDir"},"value":{"kind":"Variable","name":{"kind":"Name","value":"rootDir"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"repo"}},{"kind":"Field","name":{"kind":"Name","value":"branch"}},{"kind":"Field","name":{"kind":"Name","value":"rootDir"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}}]}}]}}]} as unknown as DocumentNode<SetRootDirMutation, SetRootDirMutationVariables>;
export const ScaleServiceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ScaleService"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"numInstances"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"scaleService"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"numInstances"},"value":{"kind":"Variable","name":{"kind":"Name","value":"numInstances"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"replicas"}}]}}]}}]} as unknown as DocumentNode<ScaleServiceMutation, ScaleServiceMutationVariables>;
export const SecretFileNamesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"SecretFileNames"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"cursor"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"limit"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"secretFiles"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"cursor"},"value":{"kind":"Variable","name":{"kind":"Name","value":"cursor"}}},{"kind":"Argument","name":{"kind":"Name","value":"limit"},"value":{"kind":"Variable","name":{"kind":"Name","value":"limit"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"secretFile"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}}]}},{"kind":"Field","name":{"kind":"Name","value":"cursor"}}]}}]}}]} as unknown as DocumentNode<SecretFileNamesQuery, SecretFileNamesQueryVariables>;
export const SecretFileContentDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"SecretFileContent"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"service"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"secretFile"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"content"}}]}}]}}]}}]} as unknown as DocumentNode<SecretFileContentQuery, SecretFileContentQueryVariables>;
export const SetSecretFileDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetSecretFile"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"content"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setSecretFile"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"content"},"value":{"kind":"Variable","name":{"kind":"Name","value":"content"}}}]}]}}]} as unknown as DocumentNode<SetSecretFileMutation, SetSecretFileMutationVariables>;
export const DeleteSecretFileDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteSecretFile"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteSecretFile"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}]}]}}]} as unknown as DocumentNode<DeleteSecretFileMutation, DeleteSecretFileMutationVariables>;
export const ServerDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Server"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"server"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"slug"}},{"kind":"Field","name":{"kind":"Name","value":"displayName"}},{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"dashboardUrl"}},{"kind":"Field","name":{"kind":"Name","value":"url"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"sshAddress"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}},{"kind":"Field","name":{"kind":"Name","value":"replicas"}},{"kind":"Field","name":{"kind":"Name","value":"revision"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"idleTTLSeconds"}},{"kind":"Field","name":{"kind":"Name","value":"maintenanceMode"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"enabled"}},{"kind":"Field","name":{"kind":"Name","value":"uri"}}]}},{"kind":"Field","name":{"kind":"Name","value":"repo"}},{"kind":"Field","name":{"kind":"Name","value":"branch"}},{"kind":"Field","name":{"kind":"Name","value":"rootDir"}},{"kind":"Field","name":{"kind":"Name","value":"runtime"}},{"kind":"Field","name":{"kind":"Name","value":"builder"}},{"kind":"Field","name":{"kind":"Name","value":"buildCommand"}},{"kind":"Field","name":{"kind":"Name","value":"startCommand"}},{"kind":"Field","name":{"kind":"Name","value":"dockerfilePath"}},{"kind":"Field","name":{"kind":"Name","value":"registryCredentialId"}},{"kind":"Field","name":{"kind":"Name","value":"buildFilter"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"paths"}},{"kind":"Field","name":{"kind":"Name","value":"ignoredPaths"}}]}},{"kind":"Field","name":{"kind":"Name","value":"autoDeploy"}},{"kind":"Field","name":{"kind":"Name","value":"notifyOnFail"}},{"kind":"Field","name":{"kind":"Name","value":"notificationsToSend"}},{"kind":"Field","name":{"kind":"Name","value":"renderSubdomainPolicy"}},{"kind":"Field","name":{"kind":"Name","value":"healthCheckPath"}},{"kind":"Field","name":{"kind":"Name","value":"maxShutdownDelaySeconds"}},{"kind":"Field","name":{"kind":"Name","value":"preDeployCommand"}},{"kind":"Field","name":{"kind":"Name","value":"schedule"}},{"kind":"Field","name":{"kind":"Name","value":"command"}},{"kind":"Field","name":{"kind":"Name","value":"lastSuccessfulRunAt"}},{"kind":"Field","name":{"kind":"Name","value":"runs"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"startedAt"}},{"kind":"Field","name":{"kind":"Name","value":"finishedAt"}},{"kind":"Field","name":{"kind":"Name","value":"status"}}]}},{"kind":"Field","name":{"kind":"Name","value":"publishPath"}},{"kind":"Field","name":{"kind":"Name","value":"routes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"source"}},{"kind":"Field","name":{"kind":"Name","value":"destination"}}]}},{"kind":"Field","name":{"kind":"Name","value":"headers"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"path"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowList"}},{"kind":"Field","name":{"kind":"Name","value":"maintenanceMode"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"enabled"}},{"kind":"Field","name":{"kind":"Name","value":"uri"}}]}}]}}]}}]} as unknown as DocumentNode<ServerQuery, ServerQueryVariables>;
export const SetNotificationsToSendDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetNotificationsToSend"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"value"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setNotificationsToSend"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"value"},"value":{"kind":"Variable","name":{"kind":"Name","value":"value"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"notificationsToSend"}},{"kind":"Field","name":{"kind":"Name","value":"notifyOnFail"}}]}}]}}]} as unknown as DocumentNode<SetNotificationsToSendMutation, SetNotificationsToSendMutationVariables>;
export const ServicesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Services"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"services"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"displayName"}},{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"dashboardUrl"}},{"kind":"Field","name":{"kind":"Name","value":"url"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}},{"kind":"Field","name":{"kind":"Name","value":"replicas"}},{"kind":"Field","name":{"kind":"Name","value":"revision"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"idleTTLSeconds"}}]}}]}}]} as unknown as DocumentNode<ServicesQuery, ServicesQueryVariables>;
export const SuspendServiceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SuspendService"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"confirm"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"suspendService"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"confirm"},"value":{"kind":"Variable","name":{"kind":"Name","value":"confirm"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}}]}}]}}]} as unknown as DocumentNode<SuspendServiceMutation, SuspendServiceMutationVariables>;
export const ResumeServiceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ResumeService"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"resumeService"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}}]}}]}}]} as unknown as DocumentNode<ResumeServiceMutation, ResumeServiceMutationVariables>;
export const DeleteServiceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteService"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"confirm"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteService"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"confirm"},"value":{"kind":"Variable","name":{"kind":"Name","value":"confirm"}}}]}]}}]} as unknown as DocumentNode<DeleteServiceMutation, DeleteServiceMutationVariables>;
export const UpdateCronJobDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"UpdateCronJob"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"schedule"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"command"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"updateCronJob"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"schedule"},"value":{"kind":"Variable","name":{"kind":"Name","value":"schedule"}}},{"kind":"Argument","name":{"kind":"Name","value":"command"},"value":{"kind":"Variable","name":{"kind":"Name","value":"command"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"schedule"}},{"kind":"Field","name":{"kind":"Name","value":"command"}}]}}]}}]} as unknown as DocumentNode<UpdateCronJobMutation, UpdateCronJobMutationVariables>;
export const CreateServiceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateService"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"environmentId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"type"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"repo"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"image"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"registryCredentialId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"branch"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"rootDir"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"runtime"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"buildCommand"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"startCommand"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"dockerfilePath"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"buildFilter"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"BuildFilterInput"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"plan"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"autoDeploy"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"schedule"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"command"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"publishPath"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"envVars"}},"type":{"kind":"ListType","type":{"kind":"NamedType","name":{"kind":"Name","value":"EnvVarInput"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"secretFiles"}},"type":{"kind":"ListType","type":{"kind":"NamedType","name":{"kind":"Name","value":"SecretFileInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createService"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}},{"kind":"Argument","name":{"kind":"Name","value":"environmentId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"environmentId"}}},{"kind":"Argument","name":{"kind":"Name","value":"type"},"value":{"kind":"Variable","name":{"kind":"Name","value":"type"}}},{"kind":"Argument","name":{"kind":"Name","value":"repo"},"value":{"kind":"Variable","name":{"kind":"Name","value":"repo"}}},{"kind":"Argument","name":{"kind":"Name","value":"image"},"value":{"kind":"Variable","name":{"kind":"Name","value":"image"}}},{"kind":"Argument","name":{"kind":"Name","value":"registryCredentialId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"registryCredentialId"}}},{"kind":"Argument","name":{"kind":"Name","value":"branch"},"value":{"kind":"Variable","name":{"kind":"Name","value":"branch"}}},{"kind":"Argument","name":{"kind":"Name","value":"rootDir"},"value":{"kind":"Variable","name":{"kind":"Name","value":"rootDir"}}},{"kind":"Argument","name":{"kind":"Name","value":"runtime"},"value":{"kind":"Variable","name":{"kind":"Name","value":"runtime"}}},{"kind":"Argument","name":{"kind":"Name","value":"buildCommand"},"value":{"kind":"Variable","name":{"kind":"Name","value":"buildCommand"}}},{"kind":"Argument","name":{"kind":"Name","value":"startCommand"},"value":{"kind":"Variable","name":{"kind":"Name","value":"startCommand"}}},{"kind":"Argument","name":{"kind":"Name","value":"dockerfilePath"},"value":{"kind":"Variable","name":{"kind":"Name","value":"dockerfilePath"}}},{"kind":"Argument","name":{"kind":"Name","value":"buildFilter"},"value":{"kind":"Variable","name":{"kind":"Name","value":"buildFilter"}}},{"kind":"Argument","name":{"kind":"Name","value":"plan"},"value":{"kind":"Variable","name":{"kind":"Name","value":"plan"}}},{"kind":"Argument","name":{"kind":"Name","value":"autoDeploy"},"value":{"kind":"Variable","name":{"kind":"Name","value":"autoDeploy"}}},{"kind":"Argument","name":{"kind":"Name","value":"schedule"},"value":{"kind":"Variable","name":{"kind":"Name","value":"schedule"}}},{"kind":"Argument","name":{"kind":"Name","value":"command"},"value":{"kind":"Variable","name":{"kind":"Name","value":"command"}}},{"kind":"Argument","name":{"kind":"Name","value":"publishPath"},"value":{"kind":"Variable","name":{"kind":"Name","value":"publishPath"}}},{"kind":"Argument","name":{"kind":"Name","value":"envVars"},"value":{"kind":"Variable","name":{"kind":"Name","value":"envVars"}}},{"kind":"Argument","name":{"kind":"Name","value":"secretFiles"},"value":{"kind":"Variable","name":{"kind":"Name","value":"secretFiles"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}},{"kind":"Field","name":{"kind":"Name","value":"projectId"}},{"kind":"Field","name":{"kind":"Name","value":"environmentId"}},{"kind":"Field","name":{"kind":"Name","value":"registryCredentialId"}},{"kind":"Field","name":{"kind":"Name","value":"latestDeployId"}}]}}]}}]} as unknown as DocumentNode<CreateServiceMutation, CreateServiceMutationVariables>;
export const SetRegistryCredentialDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetRegistryCredential"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"registryCredentialId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setRegistryCredential"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"registryCredentialId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"registryCredentialId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"registryCredentialId"}}]}}]}}]} as unknown as DocumentNode<SetRegistryCredentialMutation, SetRegistryCredentialMutationVariables>;
export const ServiceNameAvailableDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"ServiceNameAvailable"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"serviceNameAvailable"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"available"}},{"kind":"Field","name":{"kind":"Name","value":"suggestion"}}]}}]}}]} as unknown as DocumentNode<ServiceNameAvailableQuery, ServiceNameAvailableQueryVariables>;
export const SetStaticRoutesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetStaticRoutes"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"routes"}},"type":{"kind":"ListType","type":{"kind":"NamedType","name":{"kind":"Name","value":"StaticRouteInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setStaticRoutes"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"routes"},"value":{"kind":"Variable","name":{"kind":"Name","value":"routes"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"routes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"source"}},{"kind":"Field","name":{"kind":"Name","value":"destination"}}]}}]}}]}}]} as unknown as DocumentNode<SetStaticRoutesMutation, SetStaticRoutesMutationVariables>;
export const SetStaticHeadersDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetStaticHeaders"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"headers"}},"type":{"kind":"ListType","type":{"kind":"NamedType","name":{"kind":"Name","value":"StaticHeaderInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setStaticHeaders"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"headers"},"value":{"kind":"Variable","name":{"kind":"Name","value":"headers"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"headers"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"path"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}}]}}]}}]} as unknown as DocumentNode<SetStaticHeadersMutation, SetStaticHeadersMutationVariables>;
export const SetPublishPathDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetPublishPath"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"publishPath"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setPublishPath"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"publishPath"},"value":{"kind":"Variable","name":{"kind":"Name","value":"publishPath"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"publishPath"}},{"kind":"Field","name":{"kind":"Name","value":"revision"}}]}}]}}]} as unknown as DocumentNode<SetPublishPathMutation, SetPublishPathMutationVariables>;
export const SetSubdomainPolicyDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetSubdomainPolicy"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"policy"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setSubdomainPolicy"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"policy"},"value":{"kind":"Variable","name":{"kind":"Name","value":"policy"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"renderSubdomainPolicy"}}]}}]}}]} as unknown as DocumentNode<SetSubdomainPolicyMutation, SetSubdomainPolicyMutationVariables>;
export const SshKeysDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"SSHKeys"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"sshKeys"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"publicKey"}},{"kind":"Field","name":{"kind":"Name","value":"fingerprint"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<SshKeysQuery, SshKeysQueryVariables>;
export const CreateSshKeyDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateSSHKey"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"publicKey"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createSSHKey"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"publicKey"},"value":{"kind":"Variable","name":{"kind":"Name","value":"publicKey"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"publicKey"}},{"kind":"Field","name":{"kind":"Name","value":"fingerprint"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<CreateSshKeyMutation, CreateSshKeyMutationVariables>;
export const DeleteSshKeyDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteSSHKey"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteSSHKey"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<DeleteSshKeyMutation, DeleteSshKeyMutationVariables>;
export const WorkspaceMembersDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"WorkspaceMembers"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"workspaceMembers"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"subject"}},{"kind":"Field","name":{"kind":"Name","value":"userId"}},{"kind":"Field","name":{"kind":"Name","value":"email"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"mfaEnabled"}}]}},{"kind":"Field","name":{"kind":"Name","value":"workspaceSeatUsage"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"used"}},{"kind":"Field","name":{"kind":"Name","value":"limit"}}]}}]}}]} as unknown as DocumentNode<WorkspaceMembersQuery, WorkspaceMembersQueryVariables>;
export const WorkspaceInvitesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"WorkspaceInvites"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"workspaceInvites"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"email"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"expiresAt"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<WorkspaceInvitesQuery, WorkspaceInvitesQueryVariables>;
export const InviteWorkspaceMemberDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"InviteWorkspaceMember"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"email"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"role"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"inviteWorkspaceMember"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"email"},"value":{"kind":"Variable","name":{"kind":"Name","value":"email"}}},{"kind":"Argument","name":{"kind":"Name","value":"role"},"value":{"kind":"Variable","name":{"kind":"Name","value":"role"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"email"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"expiresAt"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<InviteWorkspaceMemberMutation, InviteWorkspaceMemberMutationVariables>;
export const ChangeWorkspaceMemberRoleDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ChangeWorkspaceMemberRole"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"subject"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"role"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"changeWorkspaceMemberRole"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"subject"},"value":{"kind":"Variable","name":{"kind":"Name","value":"subject"}}},{"kind":"Argument","name":{"kind":"Name","value":"role"},"value":{"kind":"Variable","name":{"kind":"Name","value":"role"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"subject"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<ChangeWorkspaceMemberRoleMutation, ChangeWorkspaceMemberRoleMutationVariables>;
export const RemoveWorkspaceMemberDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RemoveWorkspaceMember"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"subject"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"removeWorkspaceMember"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"subject"},"value":{"kind":"Variable","name":{"kind":"Name","value":"subject"}}}]}]}}]} as unknown as DocumentNode<RemoveWorkspaceMemberMutation, RemoveWorkspaceMemberMutationVariables>;
export const RevokeWorkspaceInviteDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RevokeWorkspaceInvite"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"inviteId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"revokeWorkspaceInvite"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"inviteId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"inviteId"}}}]}]}}]} as unknown as DocumentNode<RevokeWorkspaceInviteMutation, RevokeWorkspaceInviteMutationVariables>;
export const ResendWorkspaceInviteDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ResendWorkspaceInvite"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"inviteId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"resendWorkspaceInvite"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"inviteId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"inviteId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"email"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"expiresAt"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<ResendWorkspaceInviteMutation, ResendWorkspaceInviteMutationVariables>;
export const AcceptWorkspaceInviteDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"AcceptWorkspaceInvite"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"token"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"acceptWorkspaceInvite"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"token"},"value":{"kind":"Variable","name":{"kind":"Name","value":"token"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"workspaceId"}},{"kind":"Field","name":{"kind":"Name","value":"workspaceName"}},{"kind":"Field","name":{"kind":"Name","value":"role"}}]}}]}}]} as unknown as DocumentNode<AcceptWorkspaceInviteMutation, AcceptWorkspaceInviteMutationVariables>;
export const UsageDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Usage"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"period"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"usage"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"period"},"value":{"kind":"Variable","name":{"kind":"Name","value":"period"}}},{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"workspaceId"}},{"kind":"Field","name":{"kind":"Name","value":"period"}},{"kind":"Field","name":{"kind":"Name","value":"services"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"serviceId"}},{"kind":"Field","name":{"kind":"Name","value":"resourceKind"}},{"kind":"Field","name":{"kind":"Name","value":"rows"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"kind"}},{"kind":"Field","name":{"kind":"Name","value":"tier"}},{"kind":"Field","name":{"kind":"Name","value":"total"}}]}}]}},{"kind":"Field","name":{"kind":"Name","value":"estimatedCost"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"totalUsd"}},{"kind":"Field","name":{"kind":"Name","value":"meters"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"kind"}},{"kind":"Field","name":{"kind":"Name","value":"tier"}},{"kind":"Field","name":{"kind":"Name","value":"resourceKind"}},{"kind":"Field","name":{"kind":"Name","value":"costUsd"}}]}}]}}]}}]}}]} as unknown as DocumentNode<UsageQuery, UsageQueryVariables>;
export const WorkspaceLimitsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"WorkspaceLimits"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"workspaceLimits"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"services"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"used"}},{"kind":"Field","name":{"kind":"Name","value":"limit"}}]}},{"kind":"Field","name":{"kind":"Name","value":"postgres"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"used"}},{"kind":"Field","name":{"kind":"Name","value":"limit"}}]}},{"kind":"Field","name":{"kind":"Name","value":"keyValues"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"used"}},{"kind":"Field","name":{"kind":"Name","value":"limit"}}]}}]}}]}}]} as unknown as DocumentNode<WorkspaceLimitsQuery, WorkspaceLimitsQueryVariables>;
export const WebhookEndpointsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"WebhookEndpoints"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"webhookEndpoints"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"url"}},{"kind":"Field","name":{"kind":"Name","value":"eventTypes"}},{"kind":"Field","name":{"kind":"Name","value":"enabled"}},{"kind":"Field","name":{"kind":"Name","value":"disabledReason"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<WebhookEndpointsQuery, WebhookEndpointsQueryVariables>;
export const WebhookEventTypesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"WebhookEventTypes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"webhookEventTypes"}}]}}]} as unknown as DocumentNode<WebhookEventTypesQuery, WebhookEventTypesQueryVariables>;
export const WebhookDeliveriesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"WebhookDeliveries"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"endpointId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"cursor"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"limit"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"webhookDeliveries"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"endpointId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"endpointId"}}},{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}},{"kind":"Argument","name":{"kind":"Name","value":"cursor"},"value":{"kind":"Variable","name":{"kind":"Name","value":"cursor"}}},{"kind":"Argument","name":{"kind":"Name","value":"limit"},"value":{"kind":"Variable","name":{"kind":"Name","value":"limit"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"eventType"}},{"kind":"Field","name":{"kind":"Name","value":"serviceId"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"attemptCount"}},{"kind":"Field","name":{"kind":"Name","value":"lastStatusCode"}},{"kind":"Field","name":{"kind":"Name","value":"lastError"}},{"kind":"Field","name":{"kind":"Name","value":"nextAttemptAt"}},{"kind":"Field","name":{"kind":"Name","value":"deliveredAt"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"cursor"}}]}}]}}]} as unknown as DocumentNode<WebhookDeliveriesQuery, WebhookDeliveriesQueryVariables>;
export const CreateWebhookEndpointDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateWebhookEndpoint"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"url"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"eventTypes"}},"type":{"kind":"NonNullType","type":{"kind":"ListType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createWebhookEndpoint"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"url"},"value":{"kind":"Variable","name":{"kind":"Name","value":"url"}}},{"kind":"Argument","name":{"kind":"Name","value":"eventTypes"},"value":{"kind":"Variable","name":{"kind":"Name","value":"eventTypes"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"url"}},{"kind":"Field","name":{"kind":"Name","value":"eventTypes"}},{"kind":"Field","name":{"kind":"Name","value":"enabled"}},{"kind":"Field","name":{"kind":"Name","value":"secret"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<CreateWebhookEndpointMutation, CreateWebhookEndpointMutationVariables>;
export const SetWebhookEndpointEnabledDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetWebhookEndpointEnabled"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"enabled"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setWebhookEndpointEnabled"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}},{"kind":"Argument","name":{"kind":"Name","value":"enabled"},"value":{"kind":"Variable","name":{"kind":"Name","value":"enabled"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"enabled"}},{"kind":"Field","name":{"kind":"Name","value":"disabledReason"}}]}}]}}]} as unknown as DocumentNode<SetWebhookEndpointEnabledMutation, SetWebhookEndpointEnabledMutationVariables>;
export const DeleteWebhookEndpointDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteWebhookEndpoint"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteWebhookEndpoint"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}}]}]}}]} as unknown as DocumentNode<DeleteWebhookEndpointMutation, DeleteWebhookEndpointMutationVariables>;
export const WorkspacesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Workspaces"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"workspaces"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<WorkspacesQuery, WorkspacesQueryVariables>;
export const CreateWorkspaceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateWorkspace"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"plan"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createWorkspace"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"plan"},"value":{"kind":"Variable","name":{"kind":"Name","value":"plan"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<CreateWorkspaceMutation, CreateWorkspaceMutationVariables>;
export const RenameWorkspaceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RenameWorkspace"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"renameWorkspace"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<RenameWorkspaceMutation, RenameWorkspaceMutationVariables>;
export const ChangeWorkspacePlanDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ChangeWorkspacePlan"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"plan"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"changeWorkspacePlan"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"plan"},"value":{"kind":"Variable","name":{"kind":"Name","value":"plan"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<ChangeWorkspacePlanMutation, ChangeWorkspacePlanMutationVariables>;
export const DeleteWorkspaceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteWorkspace"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"confirmation"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteWorkspace"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"confirmation"},"value":{"kind":"Variable","name":{"kind":"Name","value":"confirmation"}}}]}]}}]} as unknown as DocumentNode<DeleteWorkspaceMutation, DeleteWorkspaceMutationVariables>;