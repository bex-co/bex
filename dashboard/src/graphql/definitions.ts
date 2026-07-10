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
  createdAt: Maybe<Scalars['String']['output']>;
  databaseName: Maybe<Scalars['String']['output']>;
  databaseUser: Maybe<Scalars['String']['output']>;
  diskSizeGB: Maybe<Scalars['Int']['output']>;
  externalHost: Maybe<Scalars['String']['output']>;
  highAvailabilityEnabled: Maybe<Scalars['Boolean']['output']>;
  id: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
  plan: Maybe<Scalars['String']['output']>;
  public: Maybe<Scalars['Boolean']['output']>;
  status: Maybe<Scalars['String']['output']>;
  suspended: Maybe<Scalars['String']['output']>;
  version: Maybe<Scalars['String']['output']>;
};

export type DatabaseInstanceType = {
  __typename: 'DatabaseInstanceType';
  cpu: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  memory: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
  storageGB: Maybe<Scalars['Int']['output']>;
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
  name: Maybe<Scalars['String']['output']>;
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
  createApiKey: Maybe<ApiKey>;
  createDatabase: Maybe<Database>;
  createKeyValue: Maybe<KeyValue>;
  createService: Maybe<Service>;
  createWorkspace: Maybe<Workspace>;
  deleteCustomDomain: Maybe<Scalars['Boolean']['output']>;
  deleteDatabase: Maybe<Scalars['Boolean']['output']>;
  deleteEnvVar: Maybe<Scalars['Boolean']['output']>;
  deleteKeyValue: Maybe<Scalars['Boolean']['output']>;
  deleteWorkspace: Maybe<Scalars['String']['output']>;
  renameWorkspace: Maybe<Workspace>;
  restartServer: Maybe<Service>;
  resumeKeyValue: Maybe<KeyValue>;
  resumeService: Maybe<Service>;
  revokeApiKey: Maybe<Scalars['Boolean']['output']>;
  scaleService: Maybe<Service>;
  setEnvVar: Maybe<Scalars['Boolean']['output']>;
  setEnvVars: Maybe<Scalars['Boolean']['output']>;
  setIdleTimeout: Maybe<Service>;
  setRootDir: Maybe<Service>;
  suspendKeyValue: Maybe<KeyValue>;
  suspendService: Maybe<Service>;
  updateServicePlan: Maybe<Service>;
};


export type MutationAddCustomDomainArgs = {
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
};


export type MutationCreateApiKeyArgs = {
  name: Scalars['String']['input'];
};


export type MutationCreateDatabaseArgs = {
  diskSizeGB?: InputMaybe<Scalars['Int']['input']>;
  name: Scalars['String']['input'];
  plan?: InputMaybe<Scalars['String']['input']>;
  public?: InputMaybe<Scalars['Boolean']['input']>;
  version?: InputMaybe<Scalars['String']['input']>;
};


export type MutationCreateKeyValueArgs = {
  name: Scalars['String']['input'];
  plan?: InputMaybe<Scalars['String']['input']>;
  public?: InputMaybe<Scalars['Boolean']['input']>;
  storageGB?: InputMaybe<Scalars['Int']['input']>;
  version?: InputMaybe<Scalars['String']['input']>;
};


export type MutationCreateServiceArgs = {
  autoDeploy?: InputMaybe<Scalars['Boolean']['input']>;
  branch?: InputMaybe<Scalars['String']['input']>;
  image?: InputMaybe<Scalars['String']['input']>;
  name: Scalars['String']['input'];
  plan?: InputMaybe<Scalars['String']['input']>;
  port?: InputMaybe<Scalars['Int']['input']>;
  replicas?: InputMaybe<Scalars['Int']['input']>;
  repo?: InputMaybe<Scalars['String']['input']>;
  rootDir?: InputMaybe<Scalars['String']['input']>;
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


export type MutationDeleteEnvVarArgs = {
  key: Scalars['String']['input'];
  serviceId: Scalars['String']['input'];
};


export type MutationDeleteKeyValueArgs = {
  id: Scalars['String']['input'];
};


export type MutationDeleteWorkspaceArgs = {
  confirmation: Scalars['String']['input'];
  id: Scalars['String']['input'];
};


export type MutationRenameWorkspaceArgs = {
  id: Scalars['String']['input'];
  name: Scalars['String']['input'];
};


export type MutationRestartServerArgs = {
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


export type MutationScaleServiceArgs = {
  id: Scalars['String']['input'];
  numInstances: Scalars['Int']['input'];
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


export type MutationSetRootDirArgs = {
  id: Scalars['String']['input'];
  rootDir: Scalars['String']['input'];
};


export type MutationSuspendKeyValueArgs = {
  id: Scalars['String']['input'];
};


export type MutationSuspendServiceArgs = {
  id: Scalars['String']['input'];
};


export type MutationUpdateServicePlanArgs = {
  id: Scalars['String']['input'];
  plan: Scalars['String']['input'];
};

export type PostgresConnectionInfo = {
  __typename: 'PostgresConnectionInfo';
  externalConnectionString: Maybe<Scalars['String']['output']>;
  internalConnectionString: Maybe<Scalars['String']['output']>;
  password: Maybe<Scalars['String']['output']>;
  psqlCommand: Maybe<Scalars['String']['output']>;
};

export type Query = {
  __typename: 'Query';
  apiKeys: Maybe<Array<Maybe<ApiKey>>>;
  customDomain: Maybe<CustomDomain>;
  customDomains: Maybe<Array<Maybe<CustomDomain>>>;
  database: Maybe<Database>;
  databaseConnectionInfo: Maybe<PostgresConnectionInfo>;
  databaseInstanceTypes: Maybe<Array<Maybe<DatabaseInstanceType>>>;
  databases: Maybe<Array<Maybe<Database>>>;
  instanceTypes: Maybe<Array<Maybe<InstanceType>>>;
  keyValue: Maybe<KeyValue>;
  keyValueConnectionInfo: Maybe<KeyValueConnectionInfo>;
  keyValueInstanceTypes: Maybe<Array<Maybe<KeyValueInstanceType>>>;
  keyValues: Maybe<Array<Maybe<KeyValue>>>;
  logs: Maybe<Array<Maybe<LogEntry>>>;
  metrics: Maybe<Array<Maybe<MetricSeries>>>;
  metricsFilters: Maybe<MetricsFiltersResult>;
  metricsPathFilterSuggestions: Maybe<MetricsPathFilterSuggestions>;
  monthToDateBandwidth: Maybe<MonthToDateBandwidth>;
  server: Maybe<Service>;
  service: Maybe<Service>;
  services: Maybe<Array<Maybe<Service>>>;
  usage: Maybe<UsageSummary>;
  workspaces: Maybe<Array<Maybe<Workspace>>>;
};

export type UsageRow = {
  __typename: 'UsageRow';
  kind: Maybe<Scalars['String']['output']>;
  tier: Maybe<Scalars['String']['output']>;
  total: Maybe<Scalars['Int']['output']>;
};

export type ServiceUsage = {
  __typename: 'ServiceUsage';
  serviceId: Maybe<Scalars['String']['output']>;
  rows: Maybe<Array<Maybe<UsageRow>>>;
};

export type UsageSummary = {
  __typename: 'UsageSummary';
  workspaceId: Maybe<Scalars['String']['output']>;
  services: Maybe<Array<Maybe<ServiceUsage>>>;
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


export type QueryDatabasesArgs = {
  ownerId?: InputMaybe<Scalars['String']['input']>;
};


export type QueryKeyValueArgs = {
  id: Scalars['String']['input'];
};


export type QueryKeyValueConnectionInfoArgs = {
  id: Scalars['String']['input'];
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


export type QueryServicesArgs = {
  ownerId?: InputMaybe<Scalars['String']['input']>;
};

export type Service = {
  __typename: 'Service';
  branch: Maybe<Scalars['String']['output']>;
  createdAt: Maybe<Scalars['String']['output']>;
  dashboardUrl: Maybe<Scalars['String']['output']>;
  envVar: Maybe<EnvVar>;
  envVarKeys: Maybe<Array<Maybe<EnvVar>>>;
  id: Maybe<Scalars['String']['output']>;
  idleTTLSeconds: Maybe<Scalars['Int']['output']>;
  name: Maybe<Scalars['String']['output']>;
  phase: Maybe<Scalars['String']['output']>;
  plan: Maybe<Scalars['String']['output']>;
  replicas: Maybe<Scalars['Int']['output']>;
  repo: Maybe<Scalars['String']['output']>;
  revision: Maybe<Scalars['String']['output']>;
  rootDir: Maybe<Scalars['String']['output']>;
  suspended: Maybe<Scalars['String']['output']>;
  type: Maybe<Scalars['String']['output']>;
  url: Maybe<Scalars['String']['output']>;
};


export type ServiceEnvVarArgs = {
  key: Scalars['String']['input'];
};

export type Workspace = {
  __typename: 'Workspace';
  createdAt: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
  plan: Maybe<Scalars['String']['output']>;
  role: Maybe<Scalars['String']['output']>;
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

export type WorkspaceMembersQueryVariables = Exact<{
  workspaceId: Scalars['String']['input'];
}>;


export type WorkspaceMembersQuery = { workspaceMembers: Array<{ __typename: 'WorkspaceMember', subject: string | null, role: string | null, createdAt: string | null } | null> | null };

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

export type DatabasesQueryVariables = Exact<{
  ownerId?: InputMaybe<Scalars['String']['input']>;
}>;


export type DatabasesQuery = { databases: Array<{ __typename: 'Database', id: string | null, name: string | null, plan: string | null, version: string | null, status: string | null, diskSizeGB: number | null, suspended: string | null, createdAt: string | null, public: boolean | null } | null> | null };

export type DatabaseQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DatabaseQuery = { database: { __typename: 'Database', id: string | null, name: string | null, plan: string | null, version: string | null, status: string | null, databaseName: string | null, databaseUser: string | null, diskSizeGB: number | null, highAvailabilityEnabled: boolean | null, suspended: string | null, createdAt: string | null, externalHost: string | null, public: boolean | null } | null };

export type DatabaseConnectionInfoQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DatabaseConnectionInfoQuery = { databaseConnectionInfo: { __typename: 'PostgresConnectionInfo', password: string | null, internalConnectionString: string | null, externalConnectionString: string | null, psqlCommand: string | null } | null };

export type DatabaseInstanceTypesQueryVariables = Exact<{ [key: string]: never; }>;


export type DatabaseInstanceTypesQuery = { databaseInstanceTypes: Array<{ __typename: 'DatabaseInstanceType', id: string | null, name: string | null, cpu: string | null, memory: string | null, storageGB: number | null } | null> | null };

export type CreateDatabaseMutationVariables = Exact<{
  name: Scalars['String']['input'];
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

export type KeyValuesQueryVariables = Exact<{ [key: string]: never; }>;


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

export type CreateKeyValueMutationVariables = Exact<{
  name: Scalars['String']['input'];
  plan?: InputMaybe<Scalars['String']['input']>;
  version?: InputMaybe<Scalars['String']['input']>;
  storageGB?: InputMaybe<Scalars['Int']['input']>;
  public?: InputMaybe<Scalars['Boolean']['input']>;
}>;


export type CreateKeyValueMutation = { createKeyValue: { __typename: 'KeyValue', id: string | null, name: string | null, plan: string | null, status: string | null } | null };

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

export type ServerQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type ServerQuery = { server: { __typename: 'Service', id: string | null, name: string | null, type: string | null, suspended: string | null, dashboardUrl: string | null, url: string | null, createdAt: string | null, phase: string | null, replicas: number | null, revision: string | null, plan: string | null, idleTTLSeconds: number | null, repo: string | null, branch: string | null, rootDir: string | null, schedule: string | null, command: string | null, runs: Array<{ __typename: 'CronRun', name: string | null, startedAt: string | null, finishedAt: string | null, status: string | null } | null> | null } | null };

export type SetRootDirMutationVariables = Exact<{
  id: Scalars['String']['input'];
  rootDir: Scalars['String']['input'];
}>;


export type SetRootDirMutation = { setRootDir: { __typename: 'Service', id: string | null, repo: string | null, branch: string | null, rootDir: string | null, phase: string | null } | null };

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

export type DeleteWorkspaceMutationVariables = Exact<{
  id: Scalars['String']['input'];
  confirmation: Scalars['String']['input'];
}>;


export type DeleteWorkspaceMutation = { deleteWorkspace: string | null };


export const ApiKeysDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"ApiKeys"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"apiKeys"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"createdBy"}},{"kind":"Field","name":{"kind":"Name","value":"lastUsedAt"}}]}}]}}]} as unknown as DocumentNode<ApiKeysQuery, ApiKeysQueryVariables>;
export const CreateApiKeyDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateApiKey"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createApiKey"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"secret"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<CreateApiKeyMutation, CreateApiKeyMutationVariables>;
export const RevokeApiKeyDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RevokeApiKey"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"revokeApiKey"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<RevokeApiKeyMutation, RevokeApiKeyMutationVariables>;
export const DatabasesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Databases"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databases"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"version"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"diskSizeGB"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"public"}}]}}]}}]} as unknown as DocumentNode<DatabasesQuery, DatabasesQueryVariables>;
export const DatabaseDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Database"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"database"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"version"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"databaseName"}},{"kind":"Field","name":{"kind":"Name","value":"databaseUser"}},{"kind":"Field","name":{"kind":"Name","value":"diskSizeGB"}},{"kind":"Field","name":{"kind":"Name","value":"highAvailabilityEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"externalHost"}},{"kind":"Field","name":{"kind":"Name","value":"public"}}]}}]}}]} as unknown as DocumentNode<DatabaseQuery, DatabaseQueryVariables>;
export const DatabaseConnectionInfoDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseConnectionInfo"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseConnectionInfo"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"password"}},{"kind":"Field","name":{"kind":"Name","value":"internalConnectionString"}},{"kind":"Field","name":{"kind":"Name","value":"externalConnectionString"}},{"kind":"Field","name":{"kind":"Name","value":"psqlCommand"}}]}}]}}]} as unknown as DocumentNode<DatabaseConnectionInfoQuery, DatabaseConnectionInfoQueryVariables>;
export const DatabaseInstanceTypesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"DatabaseInstanceTypes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"databaseInstanceTypes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"cpu"}},{"kind":"Field","name":{"kind":"Name","value":"memory"}},{"kind":"Field","name":{"kind":"Name","value":"storageGB"}}]}}]}}]} as unknown as DocumentNode<DatabaseInstanceTypesQuery, DatabaseInstanceTypesQueryVariables>;
export const CreateDatabaseDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateDatabase"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"plan"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"version"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"diskSizeGB"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"public"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createDatabase"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"plan"},"value":{"kind":"Variable","name":{"kind":"Name","value":"plan"}}},{"kind":"Argument","name":{"kind":"Name","value":"version"},"value":{"kind":"Variable","name":{"kind":"Name","value":"version"}}},{"kind":"Argument","name":{"kind":"Name","value":"diskSizeGB"},"value":{"kind":"Variable","name":{"kind":"Name","value":"diskSizeGB"}}},{"kind":"Argument","name":{"kind":"Name","value":"public"},"value":{"kind":"Variable","name":{"kind":"Name","value":"public"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"status"}}]}}]}}]} as unknown as DocumentNode<CreateDatabaseMutation, CreateDatabaseMutationVariables>;
export const DeleteDatabaseDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteDatabase"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteDatabase"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<DeleteDatabaseMutation, DeleteDatabaseMutationVariables>;
export const KeyValuesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"KeyValues"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"keyValues"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"version"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"externalHost"}},{"kind":"Field","name":{"kind":"Name","value":"public"}}]}}]}}]} as unknown as DocumentNode<KeyValuesQuery, KeyValuesQueryVariables>;
export const KeyValueDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"KeyValue"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"keyValue"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"version"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"externalHost"}},{"kind":"Field","name":{"kind":"Name","value":"public"}}]}}]}}]} as unknown as DocumentNode<KeyValueQuery, KeyValueQueryVariables>;
export const KeyValueConnectionInfoDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"KeyValueConnectionInfo"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"keyValueConnectionInfo"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"internalConnectionString"}},{"kind":"Field","name":{"kind":"Name","value":"externalConnectionString"}},{"kind":"Field","name":{"kind":"Name","value":"cliCommand"}}]}}]}}]} as unknown as DocumentNode<KeyValueConnectionInfoQuery, KeyValueConnectionInfoQueryVariables>;
export const KeyValueInstanceTypesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"KeyValueInstanceTypes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"keyValueInstanceTypes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"cpu"}},{"kind":"Field","name":{"kind":"Name","value":"memory"}},{"kind":"Field","name":{"kind":"Name","value":"storageGB"}}]}}]}}]} as unknown as DocumentNode<KeyValueInstanceTypesQuery, KeyValueInstanceTypesQueryVariables>;
export const CreateKeyValueDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateKeyValue"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"plan"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"version"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"storageGB"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"public"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createKeyValue"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"plan"},"value":{"kind":"Variable","name":{"kind":"Name","value":"plan"}}},{"kind":"Argument","name":{"kind":"Name","value":"version"},"value":{"kind":"Variable","name":{"kind":"Name","value":"version"}}},{"kind":"Argument","name":{"kind":"Name","value":"storageGB"},"value":{"kind":"Variable","name":{"kind":"Name","value":"storageGB"}}},{"kind":"Argument","name":{"kind":"Name","value":"public"},"value":{"kind":"Variable","name":{"kind":"Name","value":"public"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"status"}}]}}]}}]} as unknown as DocumentNode<CreateKeyValueMutation, CreateKeyValueMutationVariables>;
export const DeleteKeyValueDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteKeyValue"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteKeyValue"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<DeleteKeyValueMutation, DeleteKeyValueMutationVariables>;
export const SuspendKeyValueDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SuspendKeyValue"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"suspendKeyValue"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}}]}}]}}]} as unknown as DocumentNode<SuspendKeyValueMutation, SuspendKeyValueMutationVariables>;
export const ResumeKeyValueDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ResumeKeyValue"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"resumeKeyValue"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}}]}}]}}]} as unknown as DocumentNode<ResumeKeyValueMutation, ResumeKeyValueMutationVariables>;
export const LogsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Logs"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"resource"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"type"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"text"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"limit"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"logs"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"resource"},"value":{"kind":"Variable","name":{"kind":"Name","value":"resource"}}},{"kind":"Argument","name":{"kind":"Name","value":"type"},"value":{"kind":"Variable","name":{"kind":"Name","value":"type"}}},{"kind":"Argument","name":{"kind":"Name","value":"text"},"value":{"kind":"Variable","name":{"kind":"Name","value":"text"}}},{"kind":"Argument","name":{"kind":"Name","value":"limit"},"value":{"kind":"Variable","name":{"kind":"Name","value":"limit"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"timestamp"}},{"kind":"Field","name":{"kind":"Name","value":"message"}},{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"instance"}}]}}]}}]} as unknown as DocumentNode<LogsQuery, LogsQueryVariables>;
export const MetricsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Metrics"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"query"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"MetricsQueryInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"metrics"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"query"},"value":{"kind":"Variable","name":{"kind":"Name","value":"query"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"unit"}},{"kind":"Field","name":{"kind":"Name","value":"labels"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"field"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}},{"kind":"Field","name":{"kind":"Name","value":"values"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"time"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}},{"kind":"Field","name":{"kind":"Name","value":"parameters"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"quantile"}}]}}]}}]}}]} as unknown as DocumentNode<MetricsQuery, MetricsQueryVariables>;
export const MonthToDateBandwidthDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"MonthToDateBandwidth"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"resourceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"monthToDateBandwidth"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"resourceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"resourceId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"egressBandwidthMB"}},{"kind":"Field","name":{"kind":"Name","value":"httpEgressBandwidthMB"}},{"kind":"Field","name":{"kind":"Name","value":"natEgressBandwidthMB"}},{"kind":"Field","name":{"kind":"Name","value":"privateLinkEgressBandwidthMB"}},{"kind":"Field","name":{"kind":"Name","value":"websocketEgressBandwidthMB"}}]}}]}}]} as unknown as DocumentNode<MonthToDateBandwidthQuery, MonthToDateBandwidthQueryVariables>;
export const MetricsFiltersDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"MetricsFilters"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"query"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"MetricsFiltersQueryInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"metricsFilters"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"query"},"value":{"kind":"Variable","name":{"kind":"Name","value":"query"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"values"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"field"}},{"kind":"Field","name":{"kind":"Name","value":"values"}}]}}]}}]}}]} as unknown as DocumentNode<MetricsFiltersQuery, MetricsFiltersQueryVariables>;
export const MetricsPathFilterSuggestionsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"MetricsPathFilterSuggestions"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"query"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"MetricsPathFilterSuggestionsInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"metricsPathFilterSuggestions"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"query"},"value":{"kind":"Variable","name":{"kind":"Name","value":"query"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"paths"}}]}}]}}]} as unknown as DocumentNode<MetricsPathFilterSuggestionsQuery, MetricsPathFilterSuggestionsQueryVariables>;
export const CustomDomainsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"CustomDomains"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"customDomains"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"domainType"}},{"kind":"Field","name":{"kind":"Name","value":"verificationStatus"}},{"kind":"Field","name":{"kind":"Name","value":"serverStatus"}},{"kind":"Field","name":{"kind":"Name","value":"dnsRecord"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}}]}}]}}]} as unknown as DocumentNode<CustomDomainsQuery, CustomDomainsQueryVariables>;
export const AddCustomDomainDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"AddCustomDomain"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"addCustomDomain"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"domainType"}},{"kind":"Field","name":{"kind":"Name","value":"verificationStatus"}},{"kind":"Field","name":{"kind":"Name","value":"serverStatus"}},{"kind":"Field","name":{"kind":"Name","value":"dnsRecord"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}}]}}]}}]} as unknown as DocumentNode<AddCustomDomainMutation, AddCustomDomainMutationVariables>;
export const DeleteCustomDomainDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteCustomDomain"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteCustomDomain"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}]}]}}]} as unknown as DocumentNode<DeleteCustomDomainMutation, DeleteCustomDomainMutationVariables>;
export const VerifyCustomDomainDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"VerifyCustomDomain"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"verifyCustomDomain"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"domainType"}},{"kind":"Field","name":{"kind":"Name","value":"verificationStatus"}},{"kind":"Field","name":{"kind":"Name","value":"serverStatus"}},{"kind":"Field","name":{"kind":"Name","value":"dnsRecord"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}}]}}]}}]} as unknown as DocumentNode<VerifyCustomDomainMutation, VerifyCustomDomainMutationVariables>;
export const EnvVarKeysDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"EnvVarKeys"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"service"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"envVarKeys"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"key"}}]}}]}}]}}]} as unknown as DocumentNode<EnvVarKeysQuery, EnvVarKeysQueryVariables>;
export const EnvVarValueDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"EnvVarValue"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"key"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"service"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"envVar"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"key"},"value":{"kind":"Variable","name":{"kind":"Name","value":"key"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"key"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}}]}}]}}]} as unknown as DocumentNode<EnvVarValueQuery, EnvVarValueQueryVariables>;
export const SetEnvVarsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetEnvVars"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"envVars"}},"type":{"kind":"NonNullType","type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"EnvVarInput"}}}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setEnvVars"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"envVars"},"value":{"kind":"Variable","name":{"kind":"Name","value":"envVars"}}}]}]}}]} as unknown as DocumentNode<SetEnvVarsMutation, SetEnvVarsMutationVariables>;
export const SetEnvVarDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetEnvVar"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"key"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"value"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setEnvVar"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"key"},"value":{"kind":"Variable","name":{"kind":"Name","value":"key"}}},{"kind":"Argument","name":{"kind":"Name","value":"value"},"value":{"kind":"Variable","name":{"kind":"Name","value":"value"}}}]}]}}]} as unknown as DocumentNode<SetEnvVarMutation, SetEnvVarMutationVariables>;
export const DeleteEnvVarDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteEnvVar"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"key"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteEnvVar"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"key"},"value":{"kind":"Variable","name":{"kind":"Name","value":"key"}}}]}]}}]} as unknown as DocumentNode<DeleteEnvVarMutation, DeleteEnvVarMutationVariables>;
export const SetIdleTimeoutDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetIdleTimeout"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"idleTTLSeconds"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setIdleTimeout"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"idleTTLSeconds"},"value":{"kind":"Variable","name":{"kind":"Name","value":"idleTTLSeconds"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"idleTTLSeconds"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}}]}}]}}]} as unknown as DocumentNode<SetIdleTimeoutMutation, SetIdleTimeoutMutationVariables>;
export const InstanceTypesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"InstanceTypes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"instanceTypes"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"cpu"}},{"kind":"Field","name":{"kind":"Name","value":"memory"}}]}}]}}]} as unknown as DocumentNode<InstanceTypesQuery, InstanceTypesQueryVariables>;
export const UpdateServicePlanDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"UpdateServicePlan"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"plan"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"updateServicePlan"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"plan"},"value":{"kind":"Variable","name":{"kind":"Name","value":"plan"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}}]}}]}}]} as unknown as DocumentNode<UpdateServicePlanMutation, UpdateServicePlanMutationVariables>;
export const ServerDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Server"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"server"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"dashboardUrl"}},{"kind":"Field","name":{"kind":"Name","value":"url"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}},{"kind":"Field","name":{"kind":"Name","value":"replicas"}},{"kind":"Field","name":{"kind":"Name","value":"revision"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"idleTTLSeconds"}},{"kind":"Field","name":{"kind":"Name","value":"repo"}},{"kind":"Field","name":{"kind":"Name","value":"branch"}},{"kind":"Field","name":{"kind":"Name","value":"rootDir"}},{"kind":"Field","name":{"kind":"Name","value":"schedule"}},{"kind":"Field","name":{"kind":"Name","value":"command"}},{"kind":"Field","name":{"kind":"Name","value":"runs"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"startedAt"}},{"kind":"Field","name":{"kind":"Name","value":"finishedAt"}},{"kind":"Field","name":{"kind":"Name","value":"status"}}]}}]}}]}}]} as unknown as DocumentNode<ServerQuery, ServerQueryVariables>;

export const SetRootDirDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetRootDir"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"rootDir"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setRootDir"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"rootDir"},"value":{"kind":"Variable","name":{"kind":"Name","value":"rootDir"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"repo"}},{"kind":"Field","name":{"kind":"Name","value":"branch"}},{"kind":"Field","name":{"kind":"Name","value":"rootDir"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}}]}}]}}]} as unknown as DocumentNode<SetRootDirMutation, SetRootDirMutationVariables>;
export const ServicesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Services"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"services"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"ownerId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"ownerId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"dashboardUrl"}},{"kind":"Field","name":{"kind":"Name","value":"url"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}},{"kind":"Field","name":{"kind":"Name","value":"replicas"}},{"kind":"Field","name":{"kind":"Name","value":"revision"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"idleTTLSeconds"}}]}}]}}]} as unknown as DocumentNode<ServicesQuery, ServicesQueryVariables>;
export const SuspendServiceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SuspendService"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"suspendService"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}}]}}]}}]} as unknown as DocumentNode<SuspendServiceMutation, SuspendServiceMutationVariables>;
export const ResumeServiceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ResumeService"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"resumeService"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}}]}}]}}]} as unknown as DocumentNode<ResumeServiceMutation, ResumeServiceMutationVariables>;
export const RestartServerDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RestartServer"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"restartServer"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"suspended"}},{"kind":"Field","name":{"kind":"Name","value":"phase"}}]}}]}}]} as unknown as DocumentNode<RestartServerMutation, RestartServerMutationVariables>;
export const WorkspacesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Workspaces"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"workspaces"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<WorkspacesQuery, WorkspacesQueryVariables>;
export const CreateWorkspaceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateWorkspace"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"plan"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createWorkspace"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"plan"},"value":{"kind":"Variable","name":{"kind":"Name","value":"plan"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<CreateWorkspaceMutation, CreateWorkspaceMutationVariables>;
export const RenameWorkspaceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RenameWorkspace"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"renameWorkspace"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<RenameWorkspaceMutation, RenameWorkspaceMutationVariables>;
export const DeleteWorkspaceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteWorkspace"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"confirmation"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteWorkspace"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"confirmation"},"value":{"kind":"Variable","name":{"kind":"Name","value":"confirmation"}}}]}]}}]} as unknown as DocumentNode<DeleteWorkspaceMutation, DeleteWorkspaceMutationVariables>;
// --- w1/m16: secret files + environment groups (hand-added; codegen needs a live bex-api) ---

export type SecretFile = {
  __typename: 'SecretFile';
  content: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
};

export type EnvGroupVar = {
  __typename: 'EnvGroupVar';
  key: Maybe<Scalars['String']['output']>;
  value: Maybe<Scalars['String']['output']>;
};

export type EnvGroupSecretFile = {
  __typename: 'EnvGroupSecretFile';
  content: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
};

export type EnvGroup = {
  __typename: 'EnvGroup';
  envVars: Maybe<Array<Maybe<EnvGroupVar>>>;
  id: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
  secretFiles: Maybe<Array<Maybe<EnvGroupSecretFile>>>;
  serviceLinks: Maybe<Array<Maybe<Scalars['String']['output']>>>;
};

export type EnvGroupVarInput = {
  key: Scalars['String']['input'];
  value?: InputMaybe<Scalars['String']['input']>;
};

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

export const SecretFileNamesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"SecretFileNames"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"service"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"secretFileNames"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}}]}}]}}]}}]} as unknown as DocumentNode<SecretFileNamesQuery, SecretFileNamesQueryVariables>;
export const SecretFileContentDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"SecretFileContent"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"service"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"secretFile"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"content"}}]}}]}}]}}]} as unknown as DocumentNode<SecretFileContentQuery, SecretFileContentQueryVariables>;
export const SetSecretFileDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetSecretFile"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"content"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setSecretFile"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"content"},"value":{"kind":"Variable","name":{"kind":"Name","value":"content"}}}]}]}}]} as unknown as DocumentNode<SetSecretFileMutation, SetSecretFileMutationVariables>;
export const DeleteSecretFileDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteSecretFile"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteSecretFile"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}]}]}}]} as unknown as DocumentNode<DeleteSecretFileMutation, DeleteSecretFileMutationVariables>;
export const EnvGroupsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"EnvGroups"},"variableDefinitions":[],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"envGroups"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"serviceLinks"}},{"kind":"Field","name":{"kind":"Name","value":"envVars"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"key"}}]}},{"kind":"Field","name":{"kind":"Name","value":"secretFiles"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}}]}}]}}]}}]} as unknown as DocumentNode<EnvGroupsQuery, EnvGroupsQueryVariables>;
export const CreateEnvGroupDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateEnvGroup"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createEnvGroup"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}}]}}]}}]} as unknown as DocumentNode<CreateEnvGroupMutation, CreateEnvGroupMutationVariables>;
export const DeleteEnvGroupDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteEnvGroup"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteEnvGroup"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<DeleteEnvGroupMutation, DeleteEnvGroupMutationVariables>;
export const SetEnvGroupVarsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetEnvGroupVars"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"envVars"}},"type":{"kind":"NonNullType","type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"EnvGroupVarInput"}}}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setEnvGroupVars"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"envVars"},"value":{"kind":"Variable","name":{"kind":"Name","value":"envVars"}}}]}]}}]} as unknown as DocumentNode<SetEnvGroupVarsMutation, SetEnvGroupVarsMutationVariables>;
export const LinkEnvGroupDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"LinkEnvGroup"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"linkEnvGroup"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}}]}]}}]} as unknown as DocumentNode<LinkEnvGroupMutation, LinkEnvGroupMutationVariables>;
export const UnlinkEnvGroupDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"UnlinkEnvGroup"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"unlinkEnvGroup"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"serviceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"serviceId"}}}]}]}}]} as unknown as DocumentNode<UnlinkEnvGroupMutation, UnlinkEnvGroupMutationVariables>;
export const WorkspaceMembersDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"WorkspaceMembers"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"workspaceMembers"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"subject"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<WorkspaceMembersQuery, WorkspaceMembersQueryVariables>;
export const WorkspaceInvitesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"WorkspaceInvites"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"workspaceInvites"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"email"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"expiresAt"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<WorkspaceInvitesQuery, WorkspaceInvitesQueryVariables>;
export const InviteWorkspaceMemberDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"InviteWorkspaceMember"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"email"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"role"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"inviteWorkspaceMember"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"email"},"value":{"kind":"Variable","name":{"kind":"Name","value":"email"}}},{"kind":"Argument","name":{"kind":"Name","value":"role"},"value":{"kind":"Variable","name":{"kind":"Name","value":"role"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"email"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"expiresAt"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<InviteWorkspaceMemberMutation, InviteWorkspaceMemberMutationVariables>;
export const ChangeWorkspaceMemberRoleDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ChangeWorkspaceMemberRole"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"subject"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"role"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"changeWorkspaceMemberRole"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"subject"},"value":{"kind":"Variable","name":{"kind":"Name","value":"subject"}}},{"kind":"Argument","name":{"kind":"Name","value":"role"},"value":{"kind":"Variable","name":{"kind":"Name","value":"role"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"subject"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<ChangeWorkspaceMemberRoleMutation, ChangeWorkspaceMemberRoleMutationVariables>;
export const RemoveWorkspaceMemberDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RemoveWorkspaceMember"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"subject"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"removeWorkspaceMember"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"subject"},"value":{"kind":"Variable","name":{"kind":"Name","value":"subject"}}}]}]}}]} as unknown as DocumentNode<RemoveWorkspaceMemberMutation, RemoveWorkspaceMemberMutationVariables>;
export const RevokeWorkspaceInviteDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RevokeWorkspaceInvite"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"inviteId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"revokeWorkspaceInvite"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"inviteId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"inviteId"}}}]}]}}]} as unknown as DocumentNode<RevokeWorkspaceInviteMutation, RevokeWorkspaceInviteMutationVariables>;

export type UsageQueryVariables = Exact<{ [key: string]: never; }>;

export type UsageQuery = { usage: { __typename: 'UsageSummary', workspaceId: string | null, services: Array<{ __typename: 'ServiceUsage', serviceId: string | null, rows: Array<{ __typename: 'UsageRow', kind: string | null, tier: string | null, total: number | null } | null> | null } | null> | null } | null };

export const UsageDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Usage"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"usage"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"workspaceId"}},{"kind":"Field","name":{"kind":"Name","value":"services"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"serviceId"}},{"kind":"Field","name":{"kind":"Name","value":"rows"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"kind"}},{"kind":"Field","name":{"kind":"Name","value":"tier"}},{"kind":"Field","name":{"kind":"Name","value":"total"}}]}}]}}]}}]}}]} as unknown as DocumentNode<UsageQuery, UsageQueryVariables>;
