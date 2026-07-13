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
  resource: Maybe<Scalars['String']['output']>;
  status: Maybe<Scalars['String']['output']>;
  timestamp: Maybe<Scalars['String']['output']>;
};

export type Autoscaling = {
  __typename: 'Autoscaling';
  enabled: Maybe<Scalars['Boolean']['output']>;
  maxInstances: Maybe<Scalars['Int']['output']>;
  minInstances: Maybe<Scalars['Int']['output']>;
  targetCPUPercent: Maybe<Scalars['Int']['output']>;
  targetMemoryPercent: Maybe<Scalars['Int']['output']>;
};

export type CronRun = {
  __typename: 'CronRun';
  finishedAt: Maybe<Scalars['String']['output']>;
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
  databaseName: Maybe<Scalars['String']['output']>;
  databaseUser: Maybe<Scalars['String']['output']>;
  diskSizeGB: Maybe<Scalars['Int']['output']>;
  externalHost: Maybe<Scalars['String']['output']>;
  highAvailabilityEnabled: Maybe<Scalars['Boolean']['output']>;
  id: Maybe<Scalars['String']['output']>;
  ipAllowList: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  name: Maybe<Scalars['String']['output']>;
  ownerId: Maybe<Scalars['String']['output']>;
  plan: Maybe<Scalars['String']['output']>;
  poolerEnabled: Maybe<Scalars['Boolean']['output']>;
  public: Maybe<Scalars['Boolean']['output']>;
  readReplicas: Maybe<Array<Maybe<ReadReplicaView>>>;
  status: Maybe<Scalars['String']['output']>;
  suspended: Maybe<Scalars['String']['output']>;
  version: Maybe<Scalars['String']['output']>;
};

export type DatabaseBackup = {
  __typename: 'DatabaseBackup';
  createdAt: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  status: Maybe<Scalars['String']['output']>;
};

export type DatabaseInstanceType = {
  __typename: 'DatabaseInstanceType';
  cpu: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  memory: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
  storageGB: Maybe<Scalars['Int']['output']>;
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

export type Deploy = {
  __typename: 'Deploy';
  createdAt: Maybe<Scalars['String']['output']>;
  finishedAt: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  image: Maybe<Scalars['String']['output']>;
  rollbackOf: Maybe<Scalars['String']['output']>;
  startedAt: Maybe<Scalars['String']['output']>;
  status: Maybe<Scalars['String']['output']>;
  trigger: Maybe<Scalars['String']['output']>;
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
  envVars: Maybe<Array<Maybe<EnvGroupVar>>>;
  id: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
  secretFiles: Maybe<Array<Maybe<EnvGroupSecretFile>>>;
  serviceLinks: Maybe<Array<Maybe<Scalars['String']['output']>>>;
};

export type EnvGroupSecretFile = {
  __typename: 'EnvGroupSecretFile';
  content: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
};

export type EnvGroupVar = {
  __typename: 'EnvGroupVar';
  key: Maybe<Scalars['String']['output']>;
  value: Maybe<Scalars['String']['output']>;
};

export type EnvGroupVarInput = {
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
  key: Scalars['String']['input'];
  value?: InputMaybe<Scalars['String']['input']>;
};

export type GitConnection = {
  __typename: 'GitConnection';
  accountLogin: Maybe<Scalars['String']['output']>;
  connected: Maybe<Scalars['Boolean']['output']>;
  createdAt: Maybe<Scalars['String']['output']>;
  installUrl: Maybe<Scalars['String']['output']>;
  installationId: Maybe<Scalars['Float']['output']>;
};

export type InstanceType = {
  __typename: 'InstanceType';
  cpu: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  memory: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
};

export type KeyValue = {
  __typename: 'KeyValue';
  createdAt: Maybe<Scalars['String']['output']>;
  externalHost: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  ipAllowList: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  name: Maybe<Scalars['String']['output']>;
  ownerId: Maybe<Scalars['String']['output']>;
  plan: Maybe<Scalars['String']['output']>;
  public: Maybe<Scalars['Boolean']['output']>;
  status: Maybe<Scalars['String']['output']>;
  suspended: Maybe<Scalars['String']['output']>;
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
  message: Maybe<Scalars['String']['output']>;
  timestamp: Maybe<Scalars['String']['output']>;
  type: Maybe<Scalars['String']['output']>;
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
  addCustomDomain: Maybe<CustomDomain>;
  cancelDeploy: Maybe<Deploy>;
  changeWorkspaceMemberRole: Maybe<WorkspaceMember>;
  changeWorkspacePlan: Maybe<Workspace>;
  connectGit: Maybe<GitConnection>;
  createApiKey: Maybe<ApiKey>;
  createDatabase: Maybe<Database>;
  createDatabaseExport: Maybe<DatabaseBackup>;
  createDatabaseUser: Maybe<DatabaseUserWithPassword>;
  createEnvGroup: Maybe<EnvGroup>;
  createKeyValue: Maybe<KeyValue>;
  createService: Maybe<Service>;
  createWorkspace: Maybe<Workspace>;
  deleteCustomDomain: Maybe<Scalars['Boolean']['output']>;
  deleteDatabase: Maybe<Scalars['Boolean']['output']>;
  deleteDatabaseUser: Maybe<Scalars['Boolean']['output']>;
  deleteEnvGroup: Maybe<Scalars['Boolean']['output']>;
  deleteEnvGroupSecretFile: Maybe<Scalars['Boolean']['output']>;
  deleteEnvVar: Maybe<Scalars['Boolean']['output']>;
  deleteKeyValue: Maybe<Scalars['Boolean']['output']>;
  deleteSecretFile: Maybe<Scalars['Boolean']['output']>;
  deleteService: Maybe<Scalars['Boolean']['output']>;
  deleteWorkspace: Maybe<Scalars['String']['output']>;
  disableAutoscaling: Maybe<Scalars['Boolean']['output']>;
  disconnectGit: Maybe<Scalars['Boolean']['output']>;
  failoverDatabase: Maybe<Scalars['Boolean']['output']>;
  inviteWorkspaceMember: Maybe<WorkspaceInvite>;
  linkEnvGroup: Maybe<Scalars['Boolean']['output']>;
  recoverDatabase: Maybe<Database>;
  removeWorkspaceMember: Maybe<Scalars['String']['output']>;
  renameWorkspace: Maybe<Workspace>;
  restartDatabase: Maybe<Database>;
  restartServer: Maybe<Service>;
  resumeDatabase: Maybe<Database>;
  resumeKeyValue: Maybe<KeyValue>;
  resumeService: Maybe<Service>;
  revokeApiKey: Maybe<Scalars['Boolean']['output']>;
  revokeWorkspaceInvite: Maybe<Scalars['String']['output']>;
  rollbackService: Maybe<Deploy>;
  runCronJob: Maybe<Service>;
  scaleService: Maybe<Service>;
  setAutoDeploy: Maybe<Service>;
  setAutoscaling: Maybe<Autoscaling>;
  setDatabaseIpAllowList: Maybe<Database>;
  setDatabaseParameterOverrides: Maybe<Database>;
  setEnvGroupSecretFile: Maybe<Scalars['Boolean']['output']>;
  setEnvGroupVars: Maybe<Scalars['Boolean']['output']>;
  setEnvVar: Maybe<Scalars['Boolean']['output']>;
  setEnvVars: Maybe<Scalars['Boolean']['output']>;
  setIdleTimeout: Maybe<Service>;
  setKeyValueIpAllowList: Maybe<KeyValue>;
  setPublishPath: Maybe<Service>;
  setRootDir: Maybe<Service>;
  setSecretFile: Maybe<Scalars['Boolean']['output']>;
  setStaticHeaders: Maybe<Service>;
  setStaticRoutes: Maybe<Service>;
  suspendDatabase: Maybe<Database>;
  suspendKeyValue: Maybe<KeyValue>;
  suspendService: Maybe<Service>;
  unlinkEnvGroup: Maybe<Scalars['Boolean']['output']>;
  updateCronJob: Maybe<Service>;
  updateServicePlan: Maybe<Service>;
  verifyCustomDomain: Maybe<CustomDomain>;
};


export type MutationAddCustomDomainArgs = {
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
};


export type MutationCancelDeployArgs = {
  deployId: Scalars['String']['input'];
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


export type MutationCreateApiKeyArgs = {
  name: Scalars['String']['input'];
};


export type MutationCreateDatabaseArgs = {
  diskSizeGB?: InputMaybe<Scalars['Int']['input']>;
  enableHighAvailability?: InputMaybe<Scalars['Boolean']['input']>;
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
  name: Scalars['String']['input'];
};


export type MutationCreateKeyValueArgs = {
  ipAllowList?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  name: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
  plan?: InputMaybe<Scalars['String']['input']>;
  public?: InputMaybe<Scalars['Boolean']['input']>;
  storageGB?: InputMaybe<Scalars['Int']['input']>;
  version?: InputMaybe<Scalars['String']['input']>;
};


export type MutationCreateServiceArgs = {
  autoDeploy?: InputMaybe<Scalars['Boolean']['input']>;
  branch?: InputMaybe<Scalars['String']['input']>;
  command?: InputMaybe<Scalars['String']['input']>;
  envVars?: InputMaybe<Array<InputMaybe<EnvVarInput>>>;
  headers?: InputMaybe<Array<InputMaybe<StaticHeaderInput>>>;
  image?: InputMaybe<Scalars['String']['input']>;
  name: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
  plan?: InputMaybe<Scalars['String']['input']>;
  port?: InputMaybe<Scalars['Int']['input']>;
  publishPath?: InputMaybe<Scalars['String']['input']>;
  replicas?: InputMaybe<Scalars['Int']['input']>;
  repo?: InputMaybe<Scalars['String']['input']>;
  rootDir?: InputMaybe<Scalars['String']['input']>;
  routes?: InputMaybe<Array<InputMaybe<StaticRouteInput>>>;
  schedule?: InputMaybe<Scalars['String']['input']>;
  type?: InputMaybe<Scalars['String']['input']>;
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


export type MutationDeleteEnvVarArgs = {
  key: Scalars['String']['input'];
  serviceId: Scalars['String']['input'];
};


export type MutationDeleteKeyValueArgs = {
  id: Scalars['String']['input'];
};


export type MutationDeleteSecretFileArgs = {
  name: Scalars['String']['input'];
  serviceId: Scalars['String']['input'];
};


export type MutationDeleteServiceArgs = {
  id: Scalars['String']['input'];
};


export type MutationDeleteWorkspaceArgs = {
  confirmation: Scalars['String']['input'];
  id: Scalars['String']['input'];
};


export type MutationDisableAutoscalingArgs = {
  id: Scalars['String']['input'];
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


export type MutationRemoveWorkspaceMemberArgs = {
  subject: Scalars['String']['input'];
  workspaceId: Scalars['String']['input'];
};


export type MutationRenameWorkspaceArgs = {
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
};


export type MutationRestartDatabaseArgs = {
  id: Scalars['String']['input'];
};


export type MutationRestartServerArgs = {
  id: Scalars['String']['input'];
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


export type MutationSetDatabaseIpAllowListArgs = {
  cidrs?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  id: Scalars['String']['input'];
};


export type MutationSetDatabaseParameterOverridesArgs = {
  id: Scalars['String']['input'];
  parameters?: InputMaybe<Array<InputMaybe<ParameterInput>>>;
};


export type MutationSetEnvGroupSecretFileArgs = {
  content?: InputMaybe<Scalars['String']['input']>;
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
};


export type MutationSetEnvGroupVarsArgs = {
  envVars: Array<EnvGroupVarInput>;
  id: Scalars['String']['input'];
};


export type MutationSetEnvVarArgs = {
  key: Scalars['String']['input'];
  serviceId: Scalars['String']['input'];
  value?: InputMaybe<Scalars['String']['input']>;
};


export type MutationSetEnvVarsArgs = {
  envVars: Array<EnvVarInput>;
  serviceId: Scalars['String']['input'];
};


export type MutationSetIdleTimeoutArgs = {
  id: Scalars['String']['input'];
  idleTTLSeconds: Scalars['Int']['input'];
};


export type MutationSetKeyValueIpAllowListArgs = {
  cidrs?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  id: Scalars['String']['input'];
};


export type MutationSetPublishPathArgs = {
  id: Scalars['String']['input'];
  publishPath: Scalars['String']['input'];
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


export type MutationSetStaticHeadersArgs = {
  headers?: InputMaybe<Array<InputMaybe<StaticHeaderInput>>>;
  id: Scalars['String']['input'];
};


export type MutationSetStaticRoutesArgs = {
  id: Scalars['String']['input'];
  routes?: InputMaybe<Array<InputMaybe<StaticRouteInput>>>;
};


export type MutationSuspendDatabaseArgs = {
  id: Scalars['String']['input'];
};


export type MutationSuspendKeyValueArgs = {
  id: Scalars['String']['input'];
};


export type MutationSuspendServiceArgs = {
  id: Scalars['String']['input'];
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


export type MutationUpdateServicePlanArgs = {
  id: Scalars['String']['input'];
  plan: Scalars['String']['input'];
};


export type MutationVerifyCustomDomainArgs = {
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
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

export type Query = {
  __typename: 'Query';
  apiKeys: Maybe<Array<Maybe<ApiKey>>>;
  auditLogs: Maybe<Array<Maybe<AuditLog>>>;
  autoscalingConfig: Maybe<Autoscaling>;
  customDomain: Maybe<CustomDomain>;
  customDomains: Maybe<Array<Maybe<CustomDomain>>>;
  database: Maybe<Database>;
  databaseConnectionInfo: Maybe<PostgresConnectionInfo>;
  databaseExports: Maybe<Array<Maybe<DatabaseBackup>>>;
  databaseInstanceTypes: Maybe<Array<Maybe<DatabaseInstanceType>>>;
  databaseIpAllowList: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  databaseParameterOverrides: Maybe<Array<Maybe<DatabaseParameterOverride>>>;
  databaseProcesses: Maybe<Array<Maybe<DatabaseProcess>>>;
  databaseRecoveryInfo: Maybe<DatabaseRecoveryInfo>;
  databaseSizes: Maybe<DatabaseSizes>;
  databaseTableScans: Maybe<Array<Maybe<DatabaseTableScan>>>;
  databaseTopQueries: Maybe<Array<Maybe<DatabaseTopQuery>>>;
  databaseUsers: Maybe<Array<Maybe<DatabaseUser>>>;
  databases: Maybe<Array<Maybe<Database>>>;
  deploys: Maybe<Array<Maybe<Deploy>>>;
  envGroup: Maybe<EnvGroup>;
  envGroups: Maybe<Array<Maybe<EnvGroup>>>;
  gitConnection: Maybe<GitConnection>;
  instanceTypes: Maybe<Array<Maybe<InstanceType>>>;
  keyValue: Maybe<KeyValue>;
  keyValueConnectionInfo: Maybe<KeyValueConnectionInfo>;
  keyValueInstanceTypes: Maybe<Array<Maybe<KeyValueInstanceType>>>;
  keyValueIpAllowList: Maybe<Array<Maybe<Scalars['String']['output']>>>;
  keyValues: Maybe<Array<Maybe<KeyValue>>>;
  logs: Maybe<Array<Maybe<LogEntry>>>;
  metrics: Maybe<Array<Maybe<MetricSeries>>>;
  metricsFilters: Maybe<MetricsFiltersResult>;
  metricsPathFilterSuggestions: Maybe<MetricsPathFilterSuggestions>;
  monthToDateBandwidth: Maybe<MonthToDateBandwidth>;
  repos: Maybe<Array<Maybe<Repo>>>;
  server: Maybe<Service>;
  service: Maybe<Service>;
  serviceEvents: Maybe<Array<Maybe<ServiceEvent>>>;
  services: Maybe<Array<Maybe<Service>>>;
  usage: Maybe<UsageSummary>;
  workspaceInvites: Maybe<Array<Maybe<WorkspaceInvite>>>;
  workspaceMembers: Maybe<Array<Maybe<WorkspaceMember>>>;
  workspaces: Maybe<Array<Maybe<Workspace>>>;
};


export type QueryAuditLogsArgs = {
  cursor?: InputMaybe<Scalars['String']['input']>;
  endTime?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  ownerId: Scalars['String']['input'];
  startTime?: InputMaybe<Scalars['String']['input']>;
};


export type QueryAutoscalingConfigArgs = {
  id: Scalars['String']['input'];
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
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type QueryDeploysArgs = {
  serviceId: Scalars['String']['input'];
};


export type QueryEnvGroupArgs = {
  id: Scalars['String']['input'];
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
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type QueryLogsArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  resource: Scalars['String']['input'];
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


export type QueryServicesArgs = {
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type QueryWorkspaceInvitesArgs = {
  workspaceId: Scalars['String']['input'];
};


export type QueryWorkspaceMembersArgs = {
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

export type SecretFile = {
  __typename: 'SecretFile';
  content: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
};

export type Service = {
  __typename: 'Service';
  autoDeploy: Maybe<Scalars['Boolean']['output']>;
  autoscaling: Maybe<Autoscaling>;
  branch: Maybe<Scalars['String']['output']>;
  command: Maybe<Scalars['String']['output']>;
  createdAt: Maybe<Scalars['String']['output']>;
  dashboardUrl: Maybe<Scalars['String']['output']>;
  envVar: Maybe<EnvVar>;
  envVarKeys: Maybe<Array<Maybe<EnvVar>>>;
  headers: Maybe<Array<Maybe<StaticHeader>>>;
  id: Maybe<Scalars['String']['output']>;
  idleTTLSeconds: Maybe<Scalars['Int']['output']>;
  name: Maybe<Scalars['String']['output']>;
  ownerId: Maybe<Scalars['String']['output']>;
  phase: Maybe<Scalars['String']['output']>;
  plan: Maybe<Scalars['String']['output']>;
  publishPath: Maybe<Scalars['String']['output']>;
  replicas: Maybe<Scalars['Int']['output']>;
  repo: Maybe<Scalars['String']['output']>;
  revision: Maybe<Scalars['String']['output']>;
  rootDir: Maybe<Scalars['String']['output']>;
  routes: Maybe<Array<Maybe<StaticRoute>>>;
  runs: Maybe<Array<Maybe<CronRun>>>;
  schedule: Maybe<Scalars['String']['output']>;
  secretFile: Maybe<SecretFile>;
  secretFileNames: Maybe<Array<Maybe<SecretFile>>>;
  suspended: Maybe<Scalars['String']['output']>;
  type: Maybe<Scalars['String']['output']>;
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
  deployId: Maybe<Scalars['String']['output']>;
  deployStatus: Maybe<Scalars['String']['output']>;
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
  total: Maybe<Scalars['Int']['output']>;
};

export type UsageSummary = {
  __typename: 'UsageSummary';
  services: Maybe<Array<Maybe<ServiceUsage>>>;
  workspaceId: Maybe<Scalars['String']['output']>;
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
  role: Maybe<Scalars['String']['output']>;
  subject: Maybe<Scalars['String']['output']>;
  userId: Maybe<Scalars['String']['output']>;
};

export type ApiKeysQueryVariables = Exact<{ [key: string]: never; }>;


export type ApiKeysQuery = { apiKeys: Array<{ __typename: 'ApiKey', id: string | null, name: string | null, createdAt: string | null, createdBy: string | null, lastUsedAt: string | null } | null> | null };

export type CreateApiKeyMutationVariables = Exact<{
  name: Scalars['String']['input'];
}>;


export type CreateApiKeyMutation = { createApiKey: { __typename: 'ApiKey', id: string | null, name: string | null, secret: string | null, createdAt: string | null } | null };

export type RevokeApiKeyMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type RevokeApiKeyMutation = { revokeApiKey: boolean | null };

export type AuditLogsQueryVariables = Exact<{
  ownerId: Scalars['String']['input'];
  startTime?: InputMaybe<Scalars['String']['input']>;
  endTime?: InputMaybe<Scalars['String']['input']>;
  cursor?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
}>;


export type AuditLogsQuery = { auditLogs: Array<{ __typename: 'AuditLog', id: string | null, timestamp: string | null, actor: string | null, actorMethod: string | null, action: string | null, status: string | null, resource: string | null } | null> | null };

export type DatabasesQueryVariables = Exact<{
  ownerId?: InputMaybe<Scalars['String']['input']>;
}>;


export type DatabasesQuery = { databases: Array<{ __typename: 'Database', id: string | null, name: string | null, plan: string | null, version: string | null, status: string | null, diskSizeGB: number | null, suspended: string | null, createdAt: string | null, public: boolean | null } | null> | null };

export type DatabaseQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DatabaseQuery = { database: { __typename: 'Database', id: string | null, name: string | null, plan: string | null, version: string | null, status: string | null, databaseName: string | null, databaseUser: string | null, diskSizeGB: number | null, highAvailabilityEnabled: boolean | null, suspended: string | null, createdAt: string | null, externalHost: string | null, public: boolean | null, poolerEnabled: boolean | null, backupsEnabled: boolean | null, ipAllowList: Array<string | null> | null, readReplicas: Array<{ __typename: 'ReadReplicaView', name: string | null, connectionInfo: { __typename: 'ReadReplicaConnectionInfo', internalHost: string | null, externalHost: string | null } | null } | null> | null } | null };

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


export type DatabaseExportsQuery = { databaseExports: Array<{ __typename: 'DatabaseBackup', id: string | null, status: string | null, createdAt: string | null } | null> | null };

export type DatabaseUsersQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DatabaseUsersQuery = { databaseUsers: Array<{ __typename: 'DatabaseUser', name: string | null } | null> | null };

export type DatabaseIpAllowListQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DatabaseIpAllowListQuery = { databaseIpAllowList: Array<string | null> | null };

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


export type CreateDatabaseExportMutation = { createDatabaseExport: { __typename: 'DatabaseBackup', id: string | null, status: string | null, createdAt: string | null } | null };

export type SetDatabaseIpAllowListMutationVariables = Exact<{
  id: Scalars['String']['input'];
  cidrs?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>> | InputMaybe<Scalars['String']['input']>>;
}>;


export type SetDatabaseIpAllowListMutation = { setDatabaseIpAllowList: { __typename: 'Database', id: string | null, ipAllowList: Array<string | null> | null } | null };

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
  plan?: InputMaybe<Scalars['String']['input']>;
  version?: InputMaybe<Scalars['String']['input']>;
  diskSizeGB?: InputMaybe<Scalars['Int']['input']>;
  public?: InputMaybe<Scalars['Boolean']['input']>;
}>;


export type CreateDatabaseMutation = { createDatabase: { __typename: 'Database', id: string | null, name: string | null, plan: string | null, status: string | null } | null };

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


export type KeyValueIpAllowListQuery = { keyValueIpAllowList: Array<string | null> | null };

export type CreateKeyValueMutationVariables = Exact<{
  name: Scalars['String']['input'];
  ownerId?: InputMaybe<Scalars['String']['input']>;
  plan?: InputMaybe<Scalars['String']['input']>;
  version?: InputMaybe<Scalars['String']['input']>;
  storageGB?: InputMaybe<Scalars['Int']['input']>;
  public?: InputMaybe<Scalars['Boolean']['input']>;
}>;


export type CreateKeyValueMutation = { createKeyValue: { __typename: 'KeyValue', id: string | null, name: string | null, plan: string | null, status: string | null } | null };

export type SetKeyValueIpAllowListMutationVariables = Exact<{
  id: Scalars['String']['input'];
  cidrs?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>> | InputMaybe<Scalars['String']['input']>>;
}>;


export type SetKeyValueIpAllowListMutation = { setKeyValueIpAllowList: { __typename: 'KeyValue', id: string | null, ipAllowList: Array<string | null> | null } | null };

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
  limit?: InputMaybe<Scalars['Int']['input']>;
}>;


export type LogsQuery = { logs: Array<{ __typename: 'LogEntry', timestamp: string | null, message: string | null, type: string | null, instance: string | null } | null> | null };

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

export type CustomDomainsQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type CustomDomainsQuery = { customDomains: Array<{ __typename: 'CustomDomain', id: string | null, name: string | null, domainType: string | null, verificationStatus: string | null, serverStatus: string | null, dnsRecord: { __typename: 'DNSRecord', type: string | null, name: string | null, value: string | null } | null } | null> | null };

export type AddCustomDomainMutationVariables = Exact<{
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
}>;


export type AddCustomDomainMutation = { addCustomDomain: { __typename: 'CustomDomain', id: string | null, name: string | null, domainType: string | null, verificationStatus: string | null, serverStatus: string | null, dnsRecord: { __typename: 'DNSRecord', type: string | null, name: string | null, value: string | null } | null } | null };

export type DeleteCustomDomainMutationVariables = Exact<{
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
}>;


export type DeleteCustomDomainMutation = { deleteCustomDomain: boolean | null };

export type VerifyCustomDomainMutationVariables = Exact<{
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
}>;


export type VerifyCustomDomainMutation = { verifyCustomDomain: { __typename: 'CustomDomain', id: string | null, name: string | null, domainType: string | null, verificationStatus: string | null, serverStatus: string | null, dnsRecord: { __typename: 'DNSRecord', type: string | null, name: string | null, value: string | null } | null } | null };

export type EnvGroupsQueryVariables = Exact<{ [key: string]: never; }>;


export type EnvGroupsQuery = { envGroups: Array<{ __typename: 'EnvGroup', id: string | null, name: string | null, serviceLinks: Array<string | null> | null, envVars: Array<{ __typename: 'EnvGroupVar', key: string | null } | null> | null, secretFiles: Array<{ __typename: 'EnvGroupSecretFile', name: string | null } | null> | null } | null> | null };

export type CreateEnvGroupMutationVariables = Exact<{
  name: Scalars['String']['input'];
}>;


export type CreateEnvGroupMutation = { createEnvGroup: { __typename: 'EnvGroup', id: string | null, name: string | null } | null };

export type DeleteEnvGroupMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DeleteEnvGroupMutation = { deleteEnvGroup: boolean | null };

export type SetEnvGroupVarsMutationVariables = Exact<{
  id: Scalars['String']['input'];
  envVars: Array<EnvGroupVarInput> | EnvGroupVarInput;
}>;


export type SetEnvGroupVarsMutation = { setEnvGroupVars: boolean | null };

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

export type EnvVarKeysQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type EnvVarKeysQuery = { service: { __typename: 'Service', id: string | null, envVarKeys: Array<{ __typename: 'EnvVar', id: string | null, key: string | null } | null> | null } | null };

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
}>;


export type SetEnvVarMutation = { setEnvVar: boolean | null };

export type DeleteEnvVarMutationVariables = Exact<{
  serviceId: Scalars['String']['input'];
  key: Scalars['String']['input'];
}>;


export type DeleteEnvVarMutation = { deleteEnvVar: boolean | null };

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
  id: Scalars['String']['input'];
}>;


export type SecretFileNamesQuery = { service: { __typename: 'Service', id: string | null, secretFileNames: Array<{ __typename: 'SecretFile', id: string | null, name: string | null } | null> | null } | null };

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


export type ServerQuery = { server: { __typename: 'Service', id: string | null, name: string | null, type: string | null, suspended: string | null, dashboardUrl: string | null, url: string | null, createdAt: string | null, phase: string | null, replicas: number | null, revision: string | null, plan: string | null, idleTTLSeconds: number | null, repo: string | null, branch: string | null, rootDir: string | null, autoDeploy: boolean | null, healthCheckPath: string | null, schedule: string | null, command: string | null, publishPath: string | null, runs: Array<{ __typename: 'CronRun', name: string | null, startedAt: string | null, finishedAt: string | null, status: string | null } | null> | null, routes: Array<{ __typename: 'StaticRoute', type: string | null, source: string | null, destination: string | null } | null> | null, headers: Array<{ __typename: 'StaticHeader', path: string | null, name: string | null, value: string | null } | null> | null } | null };

export type ServicesQueryVariables = Exact<{
  ownerId?: InputMaybe<Scalars['String']['input']>;
}>;


export type ServicesQuery = { services: Array<{ __typename: 'Service', id: string | null, name: string | null, type: string | null, suspended: string | null, dashboardUrl: string | null, url: string | null, createdAt: string | null, phase: string | null, replicas: number | null, revision: string | null, plan: string | null, idleTTLSeconds: number | null } | null> | null };

export type SuspendServiceMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type SuspendServiceMutation = { suspendService: { __typename: 'Service', id: string | null, suspended: string | null, phase: string | null } | null };

export type ResumeServiceMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type ResumeServiceMutation = { resumeService: { __typename: 'Service', id: string | null, suspended: string | null, phase: string | null } | null };

export type RestartServerMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type RestartServerMutation = { restartServer: { __typename: 'Service', id: string | null, suspended: string | null, phase: string | null } | null };

export type DeleteServiceMutationVariables = Exact<{
  id: Scalars['String']['input'];
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
  type?: InputMaybe<Scalars['String']['input']>;
  repo?: InputMaybe<Scalars['String']['input']>;
  image?: InputMaybe<Scalars['String']['input']>;
  branch?: InputMaybe<Scalars['String']['input']>;
  rootDir?: InputMaybe<Scalars['String']['input']>;
  plan?: InputMaybe<Scalars['String']['input']>;
  autoDeploy?: InputMaybe<Scalars['Boolean']['input']>;
  schedule?: InputMaybe<Scalars['String']['input']>;
  command?: InputMaybe<Scalars['String']['input']>;
  publishPath?: InputMaybe<Scalars['String']['input']>;
  envVars?: InputMaybe<Array<InputMaybe<EnvVarInput>> | InputMaybe<EnvVarInput>>;
}>;


export type CreateServiceMutation = { createService: { __typename: 'Service', id: string | null, name: string | null, type: string | null, phase: string | null } | null };

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

export type WorkspaceMembersQueryVariables = Exact<{
  workspaceId: Scalars['String']['input'];
}>;


export type WorkspaceMembersQuery = { workspaceMembers: Array<{ __typename: 'WorkspaceMember', subject: string | null, userId: string | null, email: string | null, role: string | null, createdAt: string | null } | null> | null };

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

export type UsageQueryVariables = Exact<{ [key: string]: never; }>;


export type UsageQuery = { usage: { __typename: 'UsageSummary', workspaceId: string | null, services: Array<{ __typename: 'ServiceUsage', serviceId: string | null, resourceKind: string | null, rows: Array<{ __typename: 'UsageRow', kind: string | null, tier: string | null, total: number | null } | null> | null } | null> | null } | null };

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


export const ApiKeysDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"ApiKeys"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"apiKeys"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"createdBy"}},{"kind":"Field","name":{"kind":"Name","value":"lastUsedAt"}}]}}]}}]} as unknown as DocumentNode<ApiKeysQuery, ApiKeysQueryVariables>;
export const CreateApiKeyDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateApiKey"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createApiKey"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"secret"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<CreateApiKeyMutation, CreateApiKeyMutationVariables>;
export const RevokeApiKeyDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RevokeApiKey"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"revokeApiKey"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<RevokeApiKeyMutation, RevokeApiKeyMutationVariables>;
export const AuditLogsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"AuditLogs"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"startTime"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"endTime"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"cursor"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"limit"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"auditLogs"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}},{"kind":"Argument","name":{"kind":"Name","value":"startTime"},"value":{"kind":"Variable","name":{"kind":"Name","value":"startTime"}}},{"kind":"Argument","name":{"kind":"Name","value":"endTime"},"value":{"kind":"Variable","name":{"kind":"Name","value":"endTime"}}},{"kind":"Argument","name":{"kind":"Name","value":"cursor"},"value":{"kind":"Variable","name":{"kind":"Name","value":"cursor"}}},{"kind":"Argument","name":{"kind":"Name","value":"limit"},"value":{"kind":"Variable","name":{"kind":"Name","value":"limit"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"timestamp"}},{"kind":"Field","name":{"kind":"Name","value":"actor"}},{"kind":"Field","name":{"kind":"Name","value":"actorMethod"}},{"kind":"Field","name":{"kind":"Name","value":"action"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"resource"}}]}}]}}]} as unknown as DocumentNode<AuditLogsQuery, AuditLogsQueryVariables>;
export const DatabasesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Databases"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databases"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"version"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"diskSizeGB"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"public"}}]}}]}}]} as unknown as DocumentNode<DatabasesQuery, DatabasesQueryVariables>;
export const DatabaseDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Database"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"database"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"version"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"databaseName"}},{"kind":"Field","name":{"kind":"Name","value":"databaseUser"}},{"kind":"Field","name":{"kind":"Name","value":"diskSizeGB"}},{"kind":"Field","name":{"kind":"Name","value":"highAvailabilityEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"readReplicas"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"connectionInfo"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"internalHost"}},{"kind":"Field","name":{"kind":"Name","value":"externalHost"}}]}}]}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"externalHost"}},{"kind":"Field","name":{"kind":"Name","value":"public"}},{"kind":"Field","name":{"kind":"Name","value":"poolerEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"backupsEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowList"}}]}}]}}]} as unknown as DocumentNode<DatabaseQuery, DatabaseQueryVariables>;
export const DatabaseConnectionInfoDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseConnectionInfo"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseConnectionInfo"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"password"}},{"kind":"Field","name":{"kind":"Name","value":"internalConnectionString"}},{"kind":"Field","name":{"kind":"Name","value":"externalConnectionString"}},{"kind":"Field","name":{"kind":"Name","value":"internalConnectionPoolString"}},{"kind":"Field","name":{"kind":"Name","value":"externalConnectionPoolString"}},{"kind":"Field","name":{"kind":"Name","value":"psqlCommand"}},{"kind":"Field","name":{"kind":"Name","value":"readReplicaConnectionStrings"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"internalConnectionString"}},{"kind":"Field","name":{"kind":"Name","value":"externalConnectionString"}}]}}]}}]}}]} as unknown as DocumentNode<DatabaseConnectionInfoQuery, DatabaseConnectionInfoQueryVariables>;
export const DatabaseRecoveryInfoDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseRecoveryInfo"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseRecoveryInfo"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"enabled"}},{"kind":"Field","name":{"kind":"Name","value":"earliestRecoveryTime"}},{"kind":"Field","name":{"kind":"Name","value":"latestRecoveryTime"}},{"kind":"Field","name":{"kind":"Name","value":"backups"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]}}]} as unknown as DocumentNode<DatabaseRecoveryInfoQuery, DatabaseRecoveryInfoQueryVariables>;
export const DatabaseExportsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseExports"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseExports"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<DatabaseExportsQuery, DatabaseExportsQueryVariables>;
export const DatabaseUsersDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseUsers"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseUsers"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}}]}}]}}]} as unknown as DocumentNode<DatabaseUsersQuery, DatabaseUsersQueryVariables>;
export const DatabaseIpAllowListDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseIpAllowList"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseIpAllowList"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<DatabaseIpAllowListQuery, DatabaseIpAllowListQueryVariables>;
export const FailoverDatabaseDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"FailoverDatabase"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"failoverDatabase"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<FailoverDatabaseMutation, FailoverDatabaseMutationVariables>;
export const SuspendDatabaseDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SuspendDatabase"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"suspendDatabase"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"status"}}]}}]}}]} as unknown as DocumentNode<SuspendDatabaseMutation, SuspendDatabaseMutationVariables>;
export const ResumeDatabaseDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ResumeDatabase"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"resumeDatabase"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"status"}}]}}]}}]} as unknown as DocumentNode<ResumeDatabaseMutation, ResumeDatabaseMutationVariables>;
export const RestartDatabaseDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RestartDatabase"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"restartDatabase"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"status"}}]}}]}}]} as unknown as DocumentNode<RestartDatabaseMutation, RestartDatabaseMutationVariables>;
export const RecoverDatabaseDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RecoverDatabase"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"targetTime"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"plan"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"version"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"recoverDatabase"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"targetTime"},"value":{"kind":"Variable","name":{"kind":"Name","value":"targetTime"}}},{"kind":"Argument","name":{"kind":"Name","value":"plan"},"value":{"kind":"Variable","name":{"kind":"Name","value":"plan"}}},{"kind":"Argument","name":{"kind":"Name","value":"version"},"value":{"kind":"Variable","name":{"kind":"Name","value":"version"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"status"}}]}}]}}]} as unknown as DocumentNode<RecoverDatabaseMutation, RecoverDatabaseMutationVariables>;
export const CreateDatabaseExportDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateDatabaseExport"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createDatabaseExport"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<CreateDatabaseExportMutation, CreateDatabaseExportMutationVariables>;
export const SetDatabaseIpAllowListDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetDatabaseIpAllowList"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"cidrs"}},"type":{"kind":"ListType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setDatabaseIpAllowList"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"cidrs"},"value":{"kind":"Variable","name":{"kind":"Name","value":"cidrs"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowList"}}]}}]}}]} as unknown as DocumentNode<SetDatabaseIpAllowListMutation, SetDatabaseIpAllowListMutationVariables>;
export const CreateDatabaseUserDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateDatabaseUser"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createDatabaseUser"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"password"}}]}}]}}]} as unknown as DocumentNode<CreateDatabaseUserMutation, CreateDatabaseUserMutationVariables>;
export const DeleteDatabaseUserDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteDatabaseUser"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteDatabaseUser"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}]}]}}]} as unknown as DocumentNode<DeleteDatabaseUserMutation, DeleteDatabaseUserMutationVariables>;
export const DatabaseInstanceTypesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseInstanceTypes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseInstanceTypes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"cpu"}},{"kind":"Field","name":{"kind":"Name","value":"memory"}},{"kind":"Field","name":{"kind":"Name","value":"storageGB"}}]}}]}}]} as unknown as DocumentNode<DatabaseInstanceTypesQuery, DatabaseInstanceTypesQueryVariables>;
export const CreateDatabaseDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateDatabase"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"plan"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"version"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"diskSizeGB"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"public"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createDatabase"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}},{"kind":"Argument","name":{"kind":"Name","value":"plan"},"value":{"kind":"Variable","name":{"kind":"Name","value":"plan"}}},{"kind":"Argument","name":{"kind":"Name","value":"version"},"value":{"kind":"Variable","name":{"kind":"Name","value":"version"}}},{"kind":"Argument","name":{"kind":"Name","value":"diskSizeGB"},"value":{"kind":"Variable","name":{"kind":"Name","value":"diskSizeGB"}}},{"kind":"Argument","name":{"kind":"Name","value":"public"},"value":{"kind":"Variable","name":{"kind":"Name","value":"public"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"status"}}]}}]}}]} as unknown as DocumentNode<CreateDatabaseMutation, CreateDatabaseMutationVariables>;
export const DeleteDatabaseDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteDatabase"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteDatabase"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<DeleteDatabaseMutation, DeleteDatabaseMutationVariables>;
export const DatabaseProcessesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseProcesses"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseProcesses"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"pid"}},{"kind":"Field","name":{"kind":"Name","value":"userName"}},{"kind":"Field","name":{"kind":"Name","value":"applicationName"}},{"kind":"Field","name":{"kind":"Name","value":"state"}},{"kind":"Field","name":{"kind":"Name","value":"query"}},{"kind":"Field","name":{"kind":"Name","value":"waitEventType"}},{"kind":"Field","name":{"kind":"Name","value":"waitEvent"}},{"kind":"Field","name":{"kind":"Name","value":"durationSeconds"}}]}}]}}]} as unknown as DocumentNode<DatabaseProcessesQuery, DatabaseProcessesQueryVariables>;
export const DatabaseTopQueriesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseTopQueries"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseTopQueries"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"query"}},{"kind":"Field","name":{"kind":"Name","value":"calls"}},{"kind":"Field","name":{"kind":"Name","value":"totalTimeMs"}},{"kind":"Field","name":{"kind":"Name","value":"meanTimeMs"}},{"kind":"Field","name":{"kind":"Name","value":"rows"}},{"kind":"Field","name":{"kind":"Name","value":"sharedHitBlks"}},{"kind":"Field","name":{"kind":"Name","value":"sharedReadBlks"}}]}}]}}]} as unknown as DocumentNode<DatabaseTopQueriesQuery, DatabaseTopQueriesQueryVariables>;
export const DatabaseSizesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseSizes"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseSizes"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"database"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"sizeBytes"}},{"kind":"Field","name":{"kind":"Name","value":"sizePretty"}}]}},{"kind":"Field","name":{"kind":"Name","value":"tables"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"schema"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"sizeBytes"}},{"kind":"Field","name":{"kind":"Name","value":"sizePretty"}}]}}]}}]}}]} as unknown as DocumentNode<DatabaseSizesQuery, DatabaseSizesQueryVariables>;
export const DatabaseTableScansDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseTableScans"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseTableScans"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"schema"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"seqScans"}},{"kind":"Field","name":{"kind":"Name","value":"seqScanRows"}},{"kind":"Field","name":{"kind":"Name","value":"indexScans"}},{"kind":"Field","name":{"kind":"Name","value":"indexScanRows"}},{"kind":"Field","name":{"kind":"Name","value":"liveRows"}},{"kind":"Field","name":{"kind":"Name","value":"deadRows"}}]}}]}}]} as unknown as DocumentNode<DatabaseTableScansQuery, DatabaseTableScansQueryVariables>;
export const DatabaseParameterOverridesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseParameterOverrides"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseParameterOverrides"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"setting"}},{"kind":"Field","name":{"kind":"Name","value":"unit"}},{"kind":"Field","name":{"kind":"Name","value":"source"}},{"kind":"Field","name":{"kind":"Name","value":"description"}}]}}]}}]} as unknown as DocumentNode<DatabaseParameterOverridesQuery, DatabaseParameterOverridesQueryVariables>;
export const SetDatabaseParameterOverridesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetDatabaseParameterOverrides"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"parameters"}},"type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ParameterInput"}}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setDatabaseParameterOverrides"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"parameters"},"value":{"kind":"Variable","name":{"kind":"Name","value":"parameters"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}}]}}]}}]} as unknown as DocumentNode<SetDatabaseParameterOverridesMutation, SetDatabaseParameterOverridesMutationVariables>;
export const ReposDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Repos"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"repos"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"fullName"}},{"kind":"Field","name":{"kind":"Name","value":"private"}},{"kind":"Field","name":{"kind":"Name","value":"defaultBranch"}},{"kind":"Field","name":{"kind":"Name","value":"htmlUrl"}},{"kind":"Field","name":{"kind":"Name","value":"cloneUrl"}}]}}]}}]} as unknown as DocumentNode<ReposQuery, ReposQueryVariables>;
export const GitConnectionDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GitConnection"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"gitConnection"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"connected"}},{"kind":"Field","name":{"kind":"Name","value":"accountLogin"}},{"kind":"Field","name":{"kind":"Name","value":"installUrl"}}]}}]}}]} as unknown as DocumentNode<GitConnectionQuery, GitConnectionQueryVariables>;
export const ConnectGitDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ConnectGit"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"connectGit"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"connected"}},{"kind":"Field","name":{"kind":"Name","value":"installUrl"}}]}}]}}]} as unknown as DocumentNode<ConnectGitMutation, ConnectGitMutationVariables>;
export const DisconnectGitDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DisconnectGit"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"disconnectGit"}}]}}]} as unknown as DocumentNode<DisconnectGitMutation, DisconnectGitMutationVariables>;
export const KeyValuesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"KeyValues"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"keyValues"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"version"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"externalHost"}},{"kind":"Field","name":{"kind":"Name","value":"public"}}]}}]}}]} as unknown as DocumentNode<KeyValuesQuery, KeyValuesQueryVariables>;
export const KeyValueDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"KeyValue"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"keyValue"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"version"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"externalHost"}},{"kind":"Field","name":{"kind":"Name","value":"public"}}]}}]}}]} as unknown as DocumentNode<KeyValueQuery, KeyValueQueryVariables>;
export const KeyValueConnectionInfoDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"KeyValueConnectionInfo"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"keyValueConnectionInfo"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"internalConnectionString"}},{"kind":"Field","name":{"kind":"Name","value":"externalConnectionString"}},{"kind":"Field","name":{"kind":"Name","value":"cliCommand"}}]}}]}}]} as unknown as DocumentNode<KeyValueConnectionInfoQuery, KeyValueConnectionInfoQueryVariables>;
export const KeyValueInstanceTypesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"KeyValueInstanceTypes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"keyValueInstanceTypes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"cpu"}},{"kind":"Field","name":{"kind":"Name","value":"memory"}},{"kind":"Field","name":{"kind":"Name","value":"storageGB"}}]}}]}}]} as unknown as DocumentNode<KeyValueInstanceTypesQuery, KeyValueInstanceTypesQueryVariables>;
export const KeyValueIpAllowListDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"KeyValueIpAllowList"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"keyValueIpAllowList"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<KeyValueIpAllowListQuery, KeyValueIpAllowListQueryVariables>;
export const CreateKeyValueDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateKeyValue"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"plan"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"version"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"storageGB"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"public"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createKeyValue"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}},{"kind":"Argument","name":{"kind":"Name","value":"plan"},"value":{"kind":"Variable","name":{"kind":"Name","value":"plan"}}},{"kind":"Argument","name":{"kind":"Name","value":"version"},"value":{"kind":"Variable","name":{"kind":"Name","value":"version"}}},{"kind":"Argument","name":{"kind":"Name","value":"storageGB"},"value":{"kind":"Variable","name":{"kind":"Name","value":"storageGB"}}},{"kind":"Argument","name":{"kind":"Name","value":"public"},"value":{"kind":"Variable","name":{"kind":"Name","value":"public"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"status"}}]}}]}}]} as unknown as DocumentNode<CreateKeyValueMutation, CreateKeyValueMutationVariables>;
export const SetKeyValueIpAllowListDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetKeyValueIpAllowList"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"cidrs"}},"type":{"kind":"ListType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setKeyValueIpAllowList"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"cidrs"},"value":{"kind":"Variable","name":{"kind":"Name","value":"cidrs"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"ipAllowList"}}]}}]}}]} as unknown as DocumentNode<SetKeyValueIpAllowListMutation, SetKeyValueIpAllowListMutationVariables>;
export const DeleteKeyValueDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteKeyValue"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteKeyValue"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<DeleteKeyValueMutation, DeleteKeyValueMutationVariables>;
export const SuspendKeyValueDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SuspendKeyValue"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"suspendKeyValue"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}}]}}]}}]} as unknown as DocumentNode<SuspendKeyValueMutation, SuspendKeyValueMutationVariables>;
export const ResumeKeyValueDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ResumeKeyValue"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"resumeKeyValue"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}}]}}]}}]} as unknown as DocumentNode<ResumeKeyValueMutation, ResumeKeyValueMutationVariables>;
export const LogsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Logs"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"resource"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"type"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"text"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"limit"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"logs"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"resource"},"value":{"kind":"Variable","name":{"kind":"Name","value":"resource"}}},{"kind":"Argument","name":{"kind":"Name","value":"type"},"value":{"kind":"Variable","name":{"kind":"Name","value":"type"}}},{"kind":"Argument","name":{"kind":"Name","value":"text"},"value":{"kind":"Variable","name":{"kind":"Name","value":"text"}}},{"kind":"Argument","name":{"kind":"Name","value":"limit"},"value":{"kind":"Variable","name":{"kind":"Name","value":"limit"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"timestamp"}},{"kind":"Field","name":{"kind":"Name","value":"message"}},{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"instance"}}]}}]}}]} as unknown as DocumentNode<LogsQuery, LogsQueryVariables>;
export const MetricsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Metrics"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"query"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"MetricsQueryInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"metrics"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"query"},"value":{"kind":"Variable","name":{"kind":"Name","value":"query"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"unit"}},{"kind":"Field","name":{"kind":"Name","value":"labels"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"field"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}},{"kind":"Field","name":{"kind":"Name","value":"values"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"time"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}},{"kind":"Field","name":{"kind":"Name","value":"parameters"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"quantile"}}]}}]}}]}}]} as unknown as DocumentNode<MetricsQuery, MetricsQueryVariables>;
export const MonthToDateBandwidthDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"MonthToDateBandwidth"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"resourceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"monthToDateBandwidth"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"resourceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"resourceId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"egressBandwidthMB"}},{"kind":"Field","name":{"kind":"Name","value":"httpEgressBandwidthMB"}},{"kind":"Field","name":{"kind":"Name","value":"natEgressBandwidthMB"}},{"kind":"Field","name":{"kind":"Name","value":"privateLinkEgressBandwidthMB"}},{"kind":"Field","name":{"kind":"Name","value":"websocketEgressBandwidthMB"}}]}}]}}]} as unknown as DocumentNode<MonthToDateBandwidthQuery, MonthToDateBandwidthQueryVariables>;
export const MetricsFiltersDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"MetricsFilters"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"query"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"MetricsFiltersQueryInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"metricsFilters"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"query"},"value":{"kind":"Variable","name":{"kind":"Name","value":"query"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"values"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"field"}},{"kind":"Field","name":{"kind":"Name","value":"values"}}]}}]}}]}}]} as unknown as DocumentNode<MetricsFiltersQuery, MetricsFiltersQueryVariables>;
export const MetricsPathFilterSuggestionsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"MetricsPathFilterSuggestions"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"query"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"MetricsPathFilterSuggestionsInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"metricsPathFilterSuggestions"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"query"},"value":{"kind":"Variable","name":{"kind":"Name","value":"query"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"paths"}}]}}]}}]} as unknown as DocumentNode<MetricsPathFilterSuggestionsQuery, MetricsPathFilterSuggestionsQueryVariables>;
export const SetAutoDeployDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetAutoDeploy"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"enabled"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setAutoDeploy"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"enabled"},"value":{"kind":"Variable","name":{"kind":"Name","value":"enabled"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"autoDeploy"}}]}}]}}]} as unknown as DocumentNode<SetAutoDeployMutation, SetAutoDeployMutationVariables>;
export const AutoscalingConfigDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"AutoscalingConfig"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"autoscalingConfig"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"enabled"}},{"kind":"Field","name":{"kind":"Name","value":"minInstances"}},{"kind":"Field","name":{"kind":"Name","value":"maxInstances"}},{"kind":"Field","name":{"kind":"Name","value":"targetCPUPercent"}},{"kind":"Field","name":{"kind":"Name","value":"targetMemoryPercent"}}]}}]}}]} as unknown as DocumentNode<AutoscalingConfigQuery, AutoscalingConfigQueryVariables>;
export const SetAutoscalingDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetAutoscaling"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"minInstances"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"maxInstances"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"targetCPUPercent"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"targetMemoryPercent"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setAutoscaling"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"minInstances"},"value":{"kind":"Variable","name":{"kind":"Name","value":"minInstances"}}},{"kind":"Argument","name":{"kind":"Name","value":"maxInstances"},"value":{"kind":"Variable","name":{"kind":"Name","value":"maxInstances"}}},{"kind":"Argument","name":{"kind":"Name","value":"targetCPUPercent"},"value":{"kind":"Variable","name":{"kind":"Name","value":"targetCPUPercent"}}},{"kind":"Argument","name":{"kind":"Name","value":"targetMemoryPercent"},"value":{"kind":"Variable","name":{"kind":"Name","value":"targetMemoryPercent"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"enabled"}},{"kind":"Field","name":{"kind":"Name","value":"minInstances"}},{"kind":"Field","name":{"kind":"Name","value":"maxInstances"}},{"kind":"Field","name":{"kind":"Name","value":"targetCPUPercent"}},{"kind":"Field","name":{"kind":"Name","value":"targetMemoryPercent"}}]}}]}}]} as unknown as DocumentNode<SetAutoscalingMutation, SetAutoscalingMutationVariables>;
export const DisableAutoscalingDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DisableAutoscaling"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"disableAutoscaling"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<DisableAutoscalingMutation, DisableAutoscalingMutationVariables>;
export const CustomDomainsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"CustomDomains"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"customDomains"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"domainType"}},{"kind":"Field","name":{"kind":"Name","value":"verificationStatus"}},{"kind":"Field","name":{"kind":"Name","value":"serverStatus"}},{"kind":"Field","name":{"kind":"Name","value":"dnsRecord"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}}]}}]}}]} as unknown as DocumentNode<CustomDomainsQuery, CustomDomainsQueryVariables>;
export const AddCustomDomainDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"AddCustomDomain"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"addCustomDomain"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"domainType"}},{"kind":"Field","name":{"kind":"Name","value":"verificationStatus"}},{"kind":"Field","name":{"kind":"Name","value":"serverStatus"}},{"kind":"Field","name":{"kind":"Name","value":"dnsRecord"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}}]}}]}}]} as unknown as DocumentNode<AddCustomDomainMutation, AddCustomDomainMutationVariables>;
export const DeleteCustomDomainDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteCustomDomain"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteCustomDomain"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}]}]}}]} as unknown as DocumentNode<DeleteCustomDomainMutation, DeleteCustomDomainMutationVariables>;
export const VerifyCustomDomainDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"VerifyCustomDomain"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"verifyCustomDomain"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"domainType"}},{"kind":"Field","name":{"kind":"Name","value":"verificationStatus"}},{"kind":"Field","name":{"kind":"Name","value":"serverStatus"}},{"kind":"Field","name":{"kind":"Name","value":"dnsRecord"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}}]}}]}}]} as unknown as DocumentNode<VerifyCustomDomainMutation, VerifyCustomDomainMutationVariables>;
export const EnvGroupsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"EnvGroups"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"envGroups"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"serviceLinks"}},{"kind":"Field","name":{"kind":"Name","value":"envVars"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"key"}}]}},{"kind":"Field","name":{"kind":"Name","value":"secretFiles"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}}]}}]}}]}}]} as unknown as DocumentNode<EnvGroupsQuery, EnvGroupsQueryVariables>;
export const CreateEnvGroupDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateEnvGroup"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createEnvGroup"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}}]}}]}}]} as unknown as DocumentNode<CreateEnvGroupMutation, CreateEnvGroupMutationVariables>;
export const DeleteEnvGroupDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteEnvGroup"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteEnvGroup"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<DeleteEnvGroupMutation, DeleteEnvGroupMutationVariables>;
export const SetEnvGroupVarsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetEnvGroupVars"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"envVars"}},"type":{"kind":"NonNullType","type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"EnvGroupVarInput"}}}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setEnvGroupVars"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"envVars"},"value":{"kind":"Variable","name":{"kind":"Name","value":"envVars"}}}]}]}}]} as unknown as DocumentNode<SetEnvGroupVarsMutation, SetEnvGroupVarsMutationVariables>;
export const LinkEnvGroupDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"LinkEnvGroup"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"linkEnvGroup"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}}]}]}}]} as unknown as DocumentNode<LinkEnvGroupMutation, LinkEnvGroupMutationVariables>;
export const UnlinkEnvGroupDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"UnlinkEnvGroup"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"unlinkEnvGroup"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}}]}]}}]} as unknown as DocumentNode<UnlinkEnvGroupMutation, UnlinkEnvGroupMutationVariables>;
export const EnvVarKeysDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"EnvVarKeys"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"service"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"envVarKeys"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"key"}}]}}]}}]}}]} as unknown as DocumentNode<EnvVarKeysQuery, EnvVarKeysQueryVariables>;
export const EnvVarValueDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"EnvVarValue"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"key"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"service"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"envVar"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"key"},"value":{"kind":"Variable","name":{"kind":"Name","value":"key"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"key"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}}]}}]}}]} as unknown as DocumentNode<EnvVarValueQuery, EnvVarValueQueryVariables>;
export const SetEnvVarsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetEnvVars"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"envVars"}},"type":{"kind":"NonNullType","type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"EnvVarInput"}}}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setEnvVars"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"envVars"},"value":{"kind":"Variable","name":{"kind":"Name","value":"envVars"}}}]}]}}]} as unknown as DocumentNode<SetEnvVarsMutation, SetEnvVarsMutationVariables>;
export const SetEnvVarDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetEnvVar"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"key"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"value"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setEnvVar"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"key"},"value":{"kind":"Variable","name":{"kind":"Name","value":"key"}}},{"kind":"Argument","name":{"kind":"Name","value":"value"},"value":{"kind":"Variable","name":{"kind":"Name","value":"value"}}}]}]}}]} as unknown as DocumentNode<SetEnvVarMutation, SetEnvVarMutationVariables>;
export const DeleteEnvVarDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteEnvVar"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"key"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteEnvVar"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"key"},"value":{"kind":"Variable","name":{"kind":"Name","value":"key"}}}]}]}}]} as unknown as DocumentNode<DeleteEnvVarMutation, DeleteEnvVarMutationVariables>;
export const SetIdleTimeoutDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetIdleTimeout"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"idleTTLSeconds"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setIdleTimeout"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"idleTTLSeconds"},"value":{"kind":"Variable","name":{"kind":"Name","value":"idleTTLSeconds"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"idleTTLSeconds"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}}]}}]}}]} as unknown as DocumentNode<SetIdleTimeoutMutation, SetIdleTimeoutMutationVariables>;
export const InstanceTypesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"InstanceTypes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"instanceTypes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"cpu"}},{"kind":"Field","name":{"kind":"Name","value":"memory"}}]}}]}}]} as unknown as DocumentNode<InstanceTypesQuery, InstanceTypesQueryVariables>;
export const UpdateServicePlanDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"UpdateServicePlan"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"plan"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"updateServicePlan"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"plan"},"value":{"kind":"Variable","name":{"kind":"Name","value":"plan"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}}]}}]}}]} as unknown as DocumentNode<UpdateServicePlanMutation, UpdateServicePlanMutationVariables>;
export const SetRootDirDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetRootDir"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"rootDir"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setRootDir"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"rootDir"},"value":{"kind":"Variable","name":{"kind":"Name","value":"rootDir"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"repo"}},{"kind":"Field","name":{"kind":"Name","value":"branch"}},{"kind":"Field","name":{"kind":"Name","value":"rootDir"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}}]}}]}}]} as unknown as DocumentNode<SetRootDirMutation, SetRootDirMutationVariables>;
export const ScaleServiceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ScaleService"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"numInstances"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"scaleService"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"numInstances"},"value":{"kind":"Variable","name":{"kind":"Name","value":"numInstances"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"replicas"}}]}}]}}]} as unknown as DocumentNode<ScaleServiceMutation, ScaleServiceMutationVariables>;
export const SecretFileNamesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"SecretFileNames"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"service"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"secretFileNames"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}}]}}]}}]}}]} as unknown as DocumentNode<SecretFileNamesQuery, SecretFileNamesQueryVariables>;
export const SecretFileContentDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"SecretFileContent"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"service"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"secretFile"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"content"}}]}}]}}]}}]} as unknown as DocumentNode<SecretFileContentQuery, SecretFileContentQueryVariables>;
export const SetSecretFileDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetSecretFile"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"content"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setSecretFile"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"content"},"value":{"kind":"Variable","name":{"kind":"Name","value":"content"}}}]}]}}]} as unknown as DocumentNode<SetSecretFileMutation, SetSecretFileMutationVariables>;
export const DeleteSecretFileDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteSecretFile"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteSecretFile"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}]}]}}]} as unknown as DocumentNode<DeleteSecretFileMutation, DeleteSecretFileMutationVariables>;
export const ServerDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Server"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"server"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"dashboardUrl"}},{"kind":"Field","name":{"kind":"Name","value":"url"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}},{"kind":"Field","name":{"kind":"Name","value":"replicas"}},{"kind":"Field","name":{"kind":"Name","value":"revision"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"idleTTLSeconds"}},{"kind":"Field","name":{"kind":"Name","value":"repo"}},{"kind":"Field","name":{"kind":"Name","value":"branch"}},{"kind":"Field","name":{"kind":"Name","value":"rootDir"}},{"kind":"Field","name":{"kind":"Name","value":"autoDeploy"}},{"kind":"Field","name":{"kind":"Name","value":"healthCheckPath"}},{"kind":"Field","name":{"kind":"Name","value":"schedule"}},{"kind":"Field","name":{"kind":"Name","value":"command"}},{"kind":"Field","name":{"kind":"Name","value":"runs"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"startedAt"}},{"kind":"Field","name":{"kind":"Name","value":"finishedAt"}},{"kind":"Field","name":{"kind":"Name","value":"status"}}]}},{"kind":"Field","name":{"kind":"Name","value":"publishPath"}},{"kind":"Field","name":{"kind":"Name","value":"routes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"source"}},{"kind":"Field","name":{"kind":"Name","value":"destination"}}]}},{"kind":"Field","name":{"kind":"Name","value":"headers"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"path"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}}]}}]}}]} as unknown as DocumentNode<ServerQuery, ServerQueryVariables>;
export const ServicesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Services"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"services"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"dashboardUrl"}},{"kind":"Field","name":{"kind":"Name","value":"url"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}},{"kind":"Field","name":{"kind":"Name","value":"replicas"}},{"kind":"Field","name":{"kind":"Name","value":"revision"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"idleTTLSeconds"}}]}}]}}]} as unknown as DocumentNode<ServicesQuery, ServicesQueryVariables>;
export const SuspendServiceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SuspendService"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"suspendService"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}}]}}]}}]} as unknown as DocumentNode<SuspendServiceMutation, SuspendServiceMutationVariables>;
export const ResumeServiceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ResumeService"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"resumeService"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}}]}}]}}]} as unknown as DocumentNode<ResumeServiceMutation, ResumeServiceMutationVariables>;
export const RestartServerDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RestartServer"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"restartServer"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}}]}}]}}]} as unknown as DocumentNode<RestartServerMutation, RestartServerMutationVariables>;
export const DeleteServiceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteService"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteService"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<DeleteServiceMutation, DeleteServiceMutationVariables>;
export const UpdateCronJobDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"UpdateCronJob"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"schedule"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"command"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"updateCronJob"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"schedule"},"value":{"kind":"Variable","name":{"kind":"Name","value":"schedule"}}},{"kind":"Argument","name":{"kind":"Name","value":"command"},"value":{"kind":"Variable","name":{"kind":"Name","value":"command"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"schedule"}},{"kind":"Field","name":{"kind":"Name","value":"command"}}]}}]}}]} as unknown as DocumentNode<UpdateCronJobMutation, UpdateCronJobMutationVariables>;
export const CreateServiceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateService"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"type"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"repo"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"image"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"branch"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"rootDir"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"plan"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"autoDeploy"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"schedule"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"command"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"publishPath"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"envVars"}},"type":{"kind":"ListType","type":{"kind":"NamedType","name":{"kind":"Name","value":"EnvVarInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createService"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}},{"kind":"Argument","name":{"kind":"Name","value":"type"},"value":{"kind":"Variable","name":{"kind":"Name","value":"type"}}},{"kind":"Argument","name":{"kind":"Name","value":"repo"},"value":{"kind":"Variable","name":{"kind":"Name","value":"repo"}}},{"kind":"Argument","name":{"kind":"Name","value":"image"},"value":{"kind":"Variable","name":{"kind":"Name","value":"image"}}},{"kind":"Argument","name":{"kind":"Name","value":"branch"},"value":{"kind":"Variable","name":{"kind":"Name","value":"branch"}}},{"kind":"Argument","name":{"kind":"Name","value":"rootDir"},"value":{"kind":"Variable","name":{"kind":"Name","value":"rootDir"}}},{"kind":"Argument","name":{"kind":"Name","value":"plan"},"value":{"kind":"Variable","name":{"kind":"Name","value":"plan"}}},{"kind":"Argument","name":{"kind":"Name","value":"autoDeploy"},"value":{"kind":"Variable","name":{"kind":"Name","value":"autoDeploy"}}},{"kind":"Argument","name":{"kind":"Name","value":"schedule"},"value":{"kind":"Variable","name":{"kind":"Name","value":"schedule"}}},{"kind":"Argument","name":{"kind":"Name","value":"command"},"value":{"kind":"Variable","name":{"kind":"Name","value":"command"}}},{"kind":"Argument","name":{"kind":"Name","value":"publishPath"},"value":{"kind":"Variable","name":{"kind":"Name","value":"publishPath"}}},{"kind":"Argument","name":{"kind":"Name","value":"envVars"},"value":{"kind":"Variable","name":{"kind":"Name","value":"envVars"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}}]}}]}}]} as unknown as DocumentNode<CreateServiceMutation, CreateServiceMutationVariables>;
export const SetStaticRoutesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetStaticRoutes"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"routes"}},"type":{"kind":"ListType","type":{"kind":"NamedType","name":{"kind":"Name","value":"StaticRouteInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setStaticRoutes"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"routes"},"value":{"kind":"Variable","name":{"kind":"Name","value":"routes"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"routes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"source"}},{"kind":"Field","name":{"kind":"Name","value":"destination"}}]}}]}}]}}]} as unknown as DocumentNode<SetStaticRoutesMutation, SetStaticRoutesMutationVariables>;
export const SetStaticHeadersDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetStaticHeaders"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"headers"}},"type":{"kind":"ListType","type":{"kind":"NamedType","name":{"kind":"Name","value":"StaticHeaderInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setStaticHeaders"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"headers"},"value":{"kind":"Variable","name":{"kind":"Name","value":"headers"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"headers"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"path"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}}]}}]}}]} as unknown as DocumentNode<SetStaticHeadersMutation, SetStaticHeadersMutationVariables>;
export const SetPublishPathDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetPublishPath"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"publishPath"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setPublishPath"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"publishPath"},"value":{"kind":"Variable","name":{"kind":"Name","value":"publishPath"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"publishPath"}},{"kind":"Field","name":{"kind":"Name","value":"revision"}}]}}]}}]} as unknown as DocumentNode<SetPublishPathMutation, SetPublishPathMutationVariables>;
export const WorkspaceMembersDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"WorkspaceMembers"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"workspaceMembers"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"subject"}},{"kind":"Field","name":{"kind":"Name","value":"userId"}},{"kind":"Field","name":{"kind":"Name","value":"email"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<WorkspaceMembersQuery, WorkspaceMembersQueryVariables>;
export const WorkspaceInvitesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"WorkspaceInvites"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"workspaceInvites"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"email"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"expiresAt"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<WorkspaceInvitesQuery, WorkspaceInvitesQueryVariables>;
export const InviteWorkspaceMemberDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"InviteWorkspaceMember"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"email"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"role"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"inviteWorkspaceMember"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"email"},"value":{"kind":"Variable","name":{"kind":"Name","value":"email"}}},{"kind":"Argument","name":{"kind":"Name","value":"role"},"value":{"kind":"Variable","name":{"kind":"Name","value":"role"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"email"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"expiresAt"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<InviteWorkspaceMemberMutation, InviteWorkspaceMemberMutationVariables>;
export const ChangeWorkspaceMemberRoleDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ChangeWorkspaceMemberRole"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"subject"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"role"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"changeWorkspaceMemberRole"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"subject"},"value":{"kind":"Variable","name":{"kind":"Name","value":"subject"}}},{"kind":"Argument","name":{"kind":"Name","value":"role"},"value":{"kind":"Variable","name":{"kind":"Name","value":"role"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"subject"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<ChangeWorkspaceMemberRoleMutation, ChangeWorkspaceMemberRoleMutationVariables>;
export const RemoveWorkspaceMemberDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RemoveWorkspaceMember"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"subject"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"removeWorkspaceMember"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"subject"},"value":{"kind":"Variable","name":{"kind":"Name","value":"subject"}}}]}]}}]} as unknown as DocumentNode<RemoveWorkspaceMemberMutation, RemoveWorkspaceMemberMutationVariables>;
export const RevokeWorkspaceInviteDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RevokeWorkspaceInvite"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"inviteId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"revokeWorkspaceInvite"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"inviteId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"inviteId"}}}]}]}}]} as unknown as DocumentNode<RevokeWorkspaceInviteMutation, RevokeWorkspaceInviteMutationVariables>;
export const UsageDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Usage"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"usage"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"workspaceId"}},{"kind":"Field","name":{"kind":"Name","value":"services"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"serviceId"}},{"kind":"Field","name":{"kind":"Name","value":"resourceKind"}},{"kind":"Field","name":{"kind":"Name","value":"rows"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"kind"}},{"kind":"Field","name":{"kind":"Name","value":"tier"}},{"kind":"Field","name":{"kind":"Name","value":"total"}}]}}]}}]}}]}}]} as unknown as DocumentNode<UsageQuery, UsageQueryVariables>;
export const WorkspacesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Workspaces"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"workspaces"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<WorkspacesQuery, WorkspacesQueryVariables>;
export const CreateWorkspaceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateWorkspace"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"plan"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createWorkspace"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"plan"},"value":{"kind":"Variable","name":{"kind":"Name","value":"plan"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<CreateWorkspaceMutation, CreateWorkspaceMutationVariables>;
export const RenameWorkspaceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RenameWorkspace"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"renameWorkspace"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<RenameWorkspaceMutation, RenameWorkspaceMutationVariables>;
export const ChangeWorkspacePlanDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ChangeWorkspacePlan"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"plan"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"changeWorkspacePlan"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"plan"},"value":{"kind":"Variable","name":{"kind":"Name","value":"plan"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<ChangeWorkspacePlanMutation, ChangeWorkspacePlanMutationVariables>;
export const DeleteWorkspaceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteWorkspace"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"confirmation"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteWorkspace"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"confirmation"},"value":{"kind":"Variable","name":{"kind":"Name","value":"confirmation"}}}]}]}}]} as unknown as DocumentNode<DeleteWorkspaceMutation, DeleteWorkspaceMutationVariables>;

// --- w5/m7 + w1/m23: serviceEvents, triggerDeploy, cancelDeploy, rollbackService, setHealthCheckPath (hand-added; codegen needs a live bex-api) ---

export type ServiceEventsQueryVariables = Exact<{
  serviceId: Scalars['String']['input'];
  cursor?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
}>;

export type ServiceEventsQuery = { serviceEvents: { __typename: 'ServiceEventList', cursor: string | null, events: Array<{ __typename: 'ServiceEvent', id: string, type: string | null, timestamp: string | null, details: { __typename: 'EventDetails', status: string | null, image: string | null, trigger: string | null, rollbackTarget: string | null, message: string | null } | null } | null> | null } | null };

export type TriggerDeployMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;

export type TriggerDeployMutation = { triggerDeploy: { __typename: 'Deploy', id: string | null, status: string | null, createdAt: string | null, trigger: string | null, rollbackTarget: string | null, image: string | null } | null };

export type CancelDeployMutationVariables = Exact<{
  serviceId: Scalars['String']['input'];
  deployId: Scalars['String']['input'];
}>;

export type CancelDeployMutation = { cancelDeploy: { __typename: 'Deploy', id: string | null, status: string | null } | null };

export type RollbackServiceMutationVariables = Exact<{
  serviceId: Scalars['String']['input'];
  deployId: Scalars['String']['input'];
}>;

export type RollbackServiceMutation = { rollbackService: { __typename: 'Deploy', id: string | null, status: string | null, createdAt: string | null, trigger: string | null, rollbackTarget: string | null, image: string | null } | null };

export type SetHealthCheckPathMutationVariables = Exact<{
  id: Scalars['String']['input'];
  path: Scalars['String']['input'];
}>;

export type SetHealthCheckPathMutation = { setHealthCheckPath: { __typename: 'Service', id: string | null, healthCheckPath: string | null } | null };

export const ServiceEventsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"ServiceEvents"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"cursor"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"limit"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"serviceEvents"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"cursor"},"value":{"kind":"Variable","name":{"kind":"Name","value":"cursor"}}},{"kind":"Argument","name":{"kind":"Name","value":"limit"},"value":{"kind":"Variable","name":{"kind":"Name","value":"limit"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"cursor"}},{"kind":"Field","name":{"kind":"Name","value":"events"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"timestamp"}},{"kind":"Field","name":{"kind":"Name","value":"details"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"image"}},{"kind":"Field","name":{"kind":"Name","value":"trigger"}},{"kind":"Field","name":{"kind":"Name","value":"rollbackTarget"}},{"kind":"Field","name":{"kind":"Name","value":"message"}}]}}]}}]}}]}}]} as unknown as DocumentNode<ServiceEventsQuery, ServiceEventsQueryVariables>;

export const TriggerDeployDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"TriggerDeploy"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"triggerDeploy"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"trigger"}},{"kind":"Field","name":{"kind":"Name","value":"rollbackTarget"}},{"kind":"Field","name":{"kind":"Name","value":"image"}}]}}]}}]} as unknown as DocumentNode<TriggerDeployMutation, TriggerDeployMutationVariables>;

export const CancelDeployDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CancelDeploy"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"deployId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"cancelDeploy"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"deployId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"deployId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"status"}}]}}]}}]} as unknown as DocumentNode<CancelDeployMutation, CancelDeployMutationVariables>;

export const RollbackServiceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RollbackService"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"deployId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"rollbackService"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"deployId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"deployId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"trigger"}},{"kind":"Field","name":{"kind":"Name","value":"rollbackTarget"}},{"kind":"Field","name":{"kind":"Name","value":"image"}}]}}]}}]} as unknown as DocumentNode<RollbackServiceMutation, RollbackServiceMutationVariables>;

export const SetHealthCheckPathDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetHealthCheckPath"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"path"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setHealthCheckPath"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"path"},"value":{"kind":"Variable","name":{"kind":"Name","value":"path"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"healthCheckPath"}}]}}]}}]} as unknown as DocumentNode<SetHealthCheckPathMutation, SetHealthCheckPathMutationVariables>;