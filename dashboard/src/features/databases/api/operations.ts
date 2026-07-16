// Hand-written TypedDocumentNodes for the advanced managed-Postgres surface
// (w1/m17: recovery, lifecycle, access control). These MIRROR the operations in
// databases.graphql — the canonical source `yarn codegen` reads once it can
// introspect a live bex-api. Until then (codegen needs a session token, see
// codegen.ts), the panels/hooks import their typed documents from here so the
// feature type-checks and tests run offline. When codegen is next run, migrate
// these imports to `@/graphql/definitions` and delete this file.

import gql from "graphql-tag";
import type { TypedDocumentNode } from "@graphql-typed-document-node/core";

// --- shared shapes ---

export interface DatabaseBackup {
  id: string | null;
  status: string | null;
  createdAt: string | null;
}

export interface DatabaseExport extends DatabaseBackup {
  url: string | null;
  urlExpiresAt: string | null;
  expiresAt: string | null;
  filename: string | null;
  failureReason: string | null;
}

export interface DatabaseRecoveryInfo {
  enabled: boolean | null;
  earliestRecoveryTime: string | null;
  latestRecoveryTime: string | null;
  backups: Array<DatabaseBackup | null> | null;
}

export interface DatabaseUser {
  name: string | null;
}

interface IdVars {
  id: string;
}

export interface UpdateDatabaseDiskAutoscalingVars extends IdVars {
  enabled: boolean;
}

export interface UpdateDatabaseDiskAutoscalingMutation {
  updateDatabaseDiskAutoscaling: {
    id: string | null;
    diskSizeGB: number | null;
    diskAutoscalingEnabled: boolean | null;
  } | null;
}

export const UpdateDatabaseDiskAutoscalingDocument = gql`
  mutation UpdateDatabaseDiskAutoscaling($id: String!, $enabled: Boolean!) {
    updateDatabaseDiskAutoscaling(id: $id, enabled: $enabled) {
      id
      diskSizeGB
      diskAutoscalingEnabled
    }
  }
` as unknown as TypedDocumentNode<
  UpdateDatabaseDiskAutoscalingMutation,
  UpdateDatabaseDiskAutoscalingVars
>;

// --- queries ---

export interface DatabaseRecoveryInfoQuery {
  databaseRecoveryInfo: DatabaseRecoveryInfo | null;
}
export const DatabaseRecoveryInfoDocument = gql`
  query DatabaseRecoveryInfo($id: String!) {
    databaseRecoveryInfo(id: $id) {
      enabled
      earliestRecoveryTime
      latestRecoveryTime
      backups {
        id
        status
        createdAt
      }
    }
  }
` as unknown as TypedDocumentNode<DatabaseRecoveryInfoQuery, IdVars>;

export interface DatabaseExportsQuery {
  databaseExports: Array<DatabaseExport | null> | null;
}
export const DatabaseExportsDocument = gql`
  query DatabaseExports($id: String!) {
    databaseExports(id: $id) {
      id
      status
      createdAt
      url
      urlExpiresAt
      expiresAt
      filename
      failureReason
    }
  }
` as unknown as TypedDocumentNode<DatabaseExportsQuery, IdVars>;

export interface DatabaseUsersQuery {
  databaseUsers: Array<DatabaseUser | null> | null;
}
export const DatabaseUsersDocument = gql`
  query DatabaseUsers($id: String!) {
    databaseUsers(id: $id) {
      name
    }
  }
` as unknown as TypedDocumentNode<DatabaseUsersQuery, IdVars>;

// One allowlist entry: the CIDR plus the operator-facing label a human gave
// it — both persist end to end (w4/m24).
export interface IpAllowListEntry {
  cidrBlock: string;
  description: string;
}
export interface DatabaseIpAllowListQuery {
  database: {
    id: string | null;
    ipAllowListEntries: Array<{
      cidrBlock: string;
      description: string | null;
    } | null> | null;
  } | null;
}
export const DatabaseIpAllowListDocument = gql`
  query DatabaseIpAllowList($id: String!) {
    database(id: $id) {
      id
      ipAllowListEntries {
        cidrBlock
        description
      }
    }
  }
` as unknown as TypedDocumentNode<DatabaseIpAllowListQuery, IdVars>;

export interface DatabasePooledConnectionQuery {
  databaseConnectionInfo: {
    internalConnectionPoolString: string | null;
    externalConnectionPoolString: string | null;
  } | null;
}
export const DatabasePooledConnectionDocument = gql`
  query DatabasePooledConnection($id: String!) {
    databaseConnectionInfo(id: $id) {
      internalConnectionPoolString
      externalConnectionPoolString
    }
  }
` as unknown as TypedDocumentNode<DatabasePooledConnectionQuery, IdVars>;

// --- HA mutations ---

export interface FailoverDatabaseMutation {
  failoverDatabase: boolean | null;
}
export const FailoverDatabaseDocument = gql`
  mutation FailoverDatabase($id: String!) {
    failoverDatabase(id: $id)
  }
` as unknown as TypedDocumentNode<FailoverDatabaseMutation, IdVars>;

// --- lifecycle mutations ---

interface LifecycleResult {
  id: string | null;
  suspended: string | null;
  status: string | null;
}
export interface SuspendDatabaseMutation {
  suspendDatabase: LifecycleResult | null;
}
export const SuspendDatabaseDocument = gql`
  mutation SuspendDatabase($id: String!) {
    suspendDatabase(id: $id) {
      id
      suspended
      status
    }
  }
` as unknown as TypedDocumentNode<SuspendDatabaseMutation, IdVars>;

export interface ResumeDatabaseMutation {
  resumeDatabase: LifecycleResult | null;
}
export const ResumeDatabaseDocument = gql`
  mutation ResumeDatabase($id: String!) {
    resumeDatabase(id: $id) {
      id
      suspended
      status
    }
  }
` as unknown as TypedDocumentNode<ResumeDatabaseMutation, IdVars>;

export interface RestartDatabaseMutation {
  restartDatabase: LifecycleResult | null;
}
export const RestartDatabaseDocument = gql`
  mutation RestartDatabase($id: String!) {
    restartDatabase(id: $id) {
      id
      suspended
      status
    }
  }
` as unknown as TypedDocumentNode<RestartDatabaseMutation, IdVars>;

// --- recovery / exports mutations ---

export interface RecoverDatabaseVars {
  id: string;
  name: string;
  targetTime?: string | null;
  plan?: string | null;
  version?: string | null;
}
export interface RecoverDatabaseMutation {
  recoverDatabase: {
    id: string | null;
    name: string | null;
    plan: string | null;
    status: string | null;
  } | null;
}
export const RecoverDatabaseDocument = gql`
  mutation RecoverDatabase(
    $id: String!
    $name: String!
    $targetTime: String
    $plan: String
    $version: String
  ) {
    recoverDatabase(
      id: $id
      name: $name
      targetTime: $targetTime
      plan: $plan
      version: $version
    ) {
      id
      name
      plan
      status
    }
  }
` as unknown as TypedDocumentNode<RecoverDatabaseMutation, RecoverDatabaseVars>;

export interface CreateDatabaseExportMutation {
  createDatabaseExport: DatabaseExport | null;
}
export const CreateDatabaseExportDocument = gql`
  mutation CreateDatabaseExport($id: String!) {
    createDatabaseExport(id: $id) {
      id
      status
      createdAt
      url
      urlExpiresAt
      expiresAt
      filename
      failureReason
    }
  }
` as unknown as TypedDocumentNode<CreateDatabaseExportMutation, IdVars>;

// --- access mutations ---

export interface SetDatabaseIpAllowListVars {
  id: string;
  entries: IpAllowListEntry[];
}
export interface SetDatabaseIpAllowListMutation {
  setDatabaseIpAllowList: {
    id: string | null;
    ipAllowListEntries: Array<{
      cidrBlock: string;
      description: string | null;
    } | null> | null;
  } | null;
}
export const SetDatabaseIpAllowListDocument = gql`
  mutation SetDatabaseIpAllowList(
    $id: String!
    $entries: [IPAllowListEntryInput!]
  ) {
    setDatabaseIpAllowList(id: $id, entries: $entries) {
      id
      ipAllowListEntries {
        cidrBlock
        description
      }
    }
  }
` as unknown as TypedDocumentNode<
  SetDatabaseIpAllowListMutation,
  SetDatabaseIpAllowListVars
>;

export interface CreateDatabaseUserVars {
  id: string;
  name: string;
}
export interface CreateDatabaseUserMutation {
  createDatabaseUser: {
    name: string | null;
    password: string | null;
  } | null;
}
export const CreateDatabaseUserDocument = gql`
  mutation CreateDatabaseUser($id: String!, $name: String!) {
    createDatabaseUser(id: $id, name: $name) {
      name
      password
    }
  }
` as unknown as TypedDocumentNode<
  CreateDatabaseUserMutation,
  CreateDatabaseUserVars
>;

export interface DeleteDatabaseUserMutation {
  deleteDatabaseUser: boolean | null;
}
export const DeleteDatabaseUserDocument = gql`
  mutation DeleteDatabaseUser($id: String!, $name: String!) {
    deleteDatabaseUser(id: $id, name: $name)
  }
` as unknown as TypedDocumentNode<
  DeleteDatabaseUserMutation,
  CreateDatabaseUserVars
>;

// --- Insights (w2/m25) ---

export interface ProcessView {
  pid: number | null;
  userName: string | null;
  applicationName: string | null;
  state: string | null;
  query?: string | null;
  waitEventType?: string | null;
  waitEvent?: string | null;
  durationSeconds: number | null;
}
export interface DatabaseProcessesQuery {
  databaseProcesses: ProcessView[] | null;
}
export const DatabaseProcessesDocument = gql`
  query DatabaseProcesses($id: String!) {
    databaseProcesses(id: $id) {
      pid
      userName
      applicationName
      state
      query
      waitEventType
      waitEvent
      durationSeconds
    }
  }
` as unknown as TypedDocumentNode<DatabaseProcessesQuery, IdVars>;

export interface TopQueryView {
  query: string | null;
  calls: number | null;
  totalTimeMs: number | null;
  meanTimeMs: number | null;
  rows: number | null;
  sharedHitBlks: number | null;
  sharedReadBlks: number | null;
}
export interface DatabaseTopQueriesQuery {
  databaseTopQueries: TopQueryView[] | null;
}
export const DatabaseTopQueriesDocument = gql`
  query DatabaseTopQueries($id: String!) {
    databaseTopQueries(id: $id) {
      query
      calls
      totalTimeMs
      meanTimeMs
      rows
      sharedHitBlks
      sharedReadBlks
    }
  }
` as unknown as TypedDocumentNode<DatabaseTopQueriesQuery, IdVars>;

export interface DBSizeInfo {
  name: string | null;
  sizeBytes: number | null;
  sizePretty: string | null;
}
export interface TableSizeInfo {
  schema: string | null;
  name: string | null;
  sizeBytes: number | null;
  sizePretty: string | null;
}
export interface DatabaseSizesQuery {
  databaseSizes: {
    database: DBSizeInfo | null;
    tables: TableSizeInfo[] | null;
  } | null;
}
export const DatabaseSizesDocument = gql`
  query DatabaseSizes($id: String!) {
    databaseSizes(id: $id) {
      database {
        name
        sizeBytes
        sizePretty
      }
      tables {
        schema
        name
        sizeBytes
        sizePretty
      }
    }
  }
` as unknown as TypedDocumentNode<DatabaseSizesQuery, IdVars>;

export interface TableScanView {
  schema: string | null;
  name: string | null;
  seqScans: number | null;
  seqScanRows: number | null;
  indexScans: number | null;
  indexScanRows: number | null;
  liveRows: number | null;
  deadRows: number | null;
}
export interface DatabaseTableScansQuery {
  databaseTableScans: TableScanView[] | null;
}
export const DatabaseTableScansDocument = gql`
  query DatabaseTableScans($id: String!) {
    databaseTableScans(id: $id) {
      schema
      name
      seqScans
      seqScanRows
      indexScans
      indexScanRows
      liveRows
      deadRows
    }
  }
` as unknown as TypedDocumentNode<DatabaseTableScansQuery, IdVars>;

export interface ParameterOverrideView {
  name: string | null;
  setting: string | null;
  unit?: string | null;
  source: string | null;
  description?: string | null;
}
export interface DatabaseParameterOverridesQuery {
  databaseParameterOverrides: ParameterOverrideView[] | null;
}
export const DatabaseParameterOverridesDocument = gql`
  query DatabaseParameterOverrides($id: String!) {
    databaseParameterOverrides(id: $id) {
      name
      setting
      unit
      source
      description
    }
  }
` as unknown as TypedDocumentNode<DatabaseParameterOverridesQuery, IdVars>;

export interface ParameterInput {
  name: string;
  value: string;
}
export interface SetDatabaseParameterOverridesVars {
  id: string;
  parameters: ParameterInput[] | null;
}
export interface SetDatabaseParameterOverridesMutation {
  setDatabaseParameterOverrides: {
    id: string | null;
    name: string | null;
  } | null;
}
export const SetDatabaseParameterOverridesDocument = gql`
  mutation SetDatabaseParameterOverrides(
    $id: String!
    $parameters: [ParameterInput!]
  ) {
    setDatabaseParameterOverrides(id: $id, parameters: $parameters) {
      id
      name
    }
  }
` as unknown as TypedDocumentNode<
  SetDatabaseParameterOverridesMutation,
  SetDatabaseParameterOverridesVars
>;

// --- SQL console (w5/m28) ---

export interface DatabaseQueryResult {
  columns: Array<string | null> | null;
  rows: Array<{ values: Array<string | null> | null } | null> | null;
  rowCount: number | null;
  truncated: boolean | null;
}
export interface ExecuteDatabaseQueryVars {
  id: string;
  sql: string;
  allowWrites?: boolean | null;
}
export interface ExecuteDatabaseQueryMutation {
  executeDatabaseQuery: DatabaseQueryResult | null;
}
export const ExecuteDatabaseQueryDocument = gql`
  mutation ExecuteDatabaseQuery(
    $id: String!
    $sql: String!
    $allowWrites: Boolean
  ) {
    executeDatabaseQuery(id: $id, sql: $sql, allowWrites: $allowWrites) {
      columns
      rows {
        values
      }
      rowCount
      truncated
    }
  }
` as unknown as TypedDocumentNode<
  ExecuteDatabaseQueryMutation,
  ExecuteDatabaseQueryVars
>;

// --- plan update (m16) ---

export interface UpdateDatabasePlanVars {
  id: string;
  plan: string;
}
export interface UpdateDatabasePlanMutation {
  updateDatabasePlan: {
    id: string | null;
    name: string | null;
    plan: string | null;
    status: string | null;
  } | null;
}
export const UpdateDatabasePlanDocument = gql`
  mutation UpdateDatabasePlan($id: String!, $plan: String!) {
    updateDatabasePlan(id: $id, plan: $plan) {
      id
      name
      plan
      status
    }
  }
` as unknown as TypedDocumentNode<
  UpdateDatabasePlanMutation,
  UpdateDatabasePlanVars
>;

export interface UpdateDatabaseVersionVars {
  id: string;
  version: string;
}
export interface UpdateDatabaseVersionMutation {
  updateDatabaseVersion: {
    id: string | null;
    version: string | null;
    status: string | null;
  } | null;
}
export const UpdateDatabaseVersionDocument = gql`
  mutation UpdateDatabaseVersion($id: String!, $version: String!) {
    updateDatabaseVersion(id: $id, version: $version) {
      id
      version
      status
    }
  }
` as unknown as TypedDocumentNode<
  UpdateDatabaseVersionMutation,
  UpdateDatabaseVersionVars
>;

// --- display-name update (w9/m3) ---

export interface RenameDatabaseVars {
  id: string;
  name: string;
}
export interface RenameDatabaseMutation {
  renameDatabase: {
    id: string | null;
    name: string | null;
    databaseName: string | null;
    databaseUser: string | null;
    externalHost: string | null;
  } | null;
}
export const RenameDatabaseDocument = gql`
  mutation RenameDatabase($id: String!, $name: String!) {
    renameDatabase(id: $id, name: $name) {
      id
      name
      databaseName
      databaseUser
      externalHost
    }
  }
` as unknown as TypedDocumentNode<RenameDatabaseMutation, RenameDatabaseVars>;
