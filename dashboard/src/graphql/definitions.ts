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
  id: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
  secret: Maybe<Scalars['String']['output']>;
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

export type MetricPoint = {
  __typename: 'MetricPoint';
  timestamp: Maybe<Scalars['String']['output']>;
  value: Maybe<Scalars['Float']['output']>;
};

export type MetricSeries = {
  __typename: 'MetricSeries';
  labels: Maybe<Array<Maybe<MetricLabel>>>;
  points: Maybe<Array<Maybe<MetricPoint>>>;
  unit: Maybe<Scalars['String']['output']>;
};

export type Mutation = {
  __typename: 'Mutation';
  createApiKey: Maybe<ApiKey>;
  createDatabase: Maybe<Database>;
  deleteDatabase: Maybe<Scalars['Boolean']['output']>;
  restartServer: Maybe<Service>;
  resumeService: Maybe<Service>;
  revokeApiKey: Maybe<Scalars['Boolean']['output']>;
  suspendService: Maybe<Service>;
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


export type MutationDeleteDatabaseArgs = {
  id: Scalars['String']['input'];
};


export type MutationRestartServerArgs = {
  id: Scalars['String']['input'];
};


export type MutationResumeServiceArgs = {
  id: Scalars['String']['input'];
};


export type MutationRevokeApiKeyArgs = {
  id: Scalars['String']['input'];
};


export type MutationSuspendServiceArgs = {
  id: Scalars['String']['input'];
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
  database: Maybe<Database>;
  databaseConnectionInfo: Maybe<PostgresConnectionInfo>;
  databases: Maybe<Array<Maybe<Database>>>;
  logs: Maybe<Array<Maybe<LogEntry>>>;
  metrics: Maybe<Array<Maybe<MetricSeries>>>;
  server: Maybe<Service>;
  services: Maybe<Array<Maybe<Service>>>;
};


export type QueryDatabaseArgs = {
  id: Scalars['String']['input'];
};


export type QueryDatabaseConnectionInfoArgs = {
  id: Scalars['String']['input'];
};


export type QueryLogsArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  resource: Scalars['String']['input'];
  text?: InputMaybe<Scalars['String']['input']>;
  type?: InputMaybe<Scalars['String']['input']>;
};


export type QueryMetricsArgs = {
  endTime?: InputMaybe<Scalars['String']['input']>;
  groupBy?: InputMaybe<Scalars['String']['input']>;
  host?: InputMaybe<Scalars['String']['input']>;
  metric: Scalars['String']['input'];
  path?: InputMaybe<Scalars['String']['input']>;
  percentage?: InputMaybe<Scalars['Boolean']['input']>;
  quantile?: InputMaybe<Scalars['Float']['input']>;
  resolutionSeconds?: InputMaybe<Scalars['Int']['input']>;
  resource: Scalars['String']['input'];
  startTime?: InputMaybe<Scalars['String']['input']>;
  statusCode?: InputMaybe<Scalars['String']['input']>;
};


export type QueryServerArgs = {
  id: Scalars['String']['input'];
};

export type Service = {
  __typename: 'Service';
  createdAt: Maybe<Scalars['String']['output']>;
  dashboardUrl: Maybe<Scalars['String']['output']>;
  id: Maybe<Scalars['String']['output']>;
  name: Maybe<Scalars['String']['output']>;
  phase: Maybe<Scalars['String']['output']>;
  replicas: Maybe<Scalars['Int']['output']>;
  revision: Maybe<Scalars['String']['output']>;
  suspended: Maybe<Scalars['String']['output']>;
  type: Maybe<Scalars['String']['output']>;
  url: Maybe<Scalars['String']['output']>;
};

export type MetricsQueryVariables = Exact<{
  resource: Scalars['String']['input'];
  metric: Scalars['String']['input'];
  startTime?: InputMaybe<Scalars['String']['input']>;
  endTime?: InputMaybe<Scalars['String']['input']>;
  resolutionSeconds?: InputMaybe<Scalars['Int']['input']>;
  quantile?: InputMaybe<Scalars['Float']['input']>;
  percentage?: InputMaybe<Scalars['Boolean']['input']>;
}>;


export type MetricsQuery = { metrics: Array<{ __typename: 'MetricSeries', unit: string | null, labels: Array<{ __typename: 'MetricLabel', field: string | null, value: string | null } | null> | null, points: Array<{ __typename: 'MetricPoint', timestamp: string | null, value: number | null } | null> | null } | null> | null };


export const MetricsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Metrics"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"resource"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"metric"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"startTime"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"endTime"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"resolutionSeconds"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"quantile"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Float"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"percentage"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"metrics"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"resource"},"value":{"kind":"Variable","name":{"kind":"Name","value":"resource"}}},{"kind":"Argument","name":{"kind":"Name","value":"metric"},"value":{"kind":"Variable","name":{"kind":"Name","value":"metric"}}},{"kind":"Argument","name":{"kind":"Name","value":"startTime"},"value":{"kind":"Variable","name":{"kind":"Name","value":"startTime"}}},{"kind":"Argument","name":{"kind":"Name","value":"endTime"},"value":{"kind":"Variable","name":{"kind":"Name","value":"endTime"}}},{"kind":"Argument","name":{"kind":"Name","value":"resolutionSeconds"},"value":{"kind":"Variable","name":{"kind":"Name","value":"resolutionSeconds"}}},{"kind":"Argument","name":{"kind":"Name","value":"quantile"},"value":{"kind":"Variable","name":{"kind":"Name","value":"quantile"}}},{"kind":"Argument","name":{"kind":"Name","value":"percentage"},"value":{"kind":"Variable","name":{"kind":"Name","value":"percentage"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"unit"}},{"kind":"Field","name":{"kind":"Name","value":"labels"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"field"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}},{"kind":"Field","name":{"kind":"Name","value":"points"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"timestamp"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}}]}}]}}]} as unknown as DocumentNode<MetricsQuery, MetricsQueryVariables>;