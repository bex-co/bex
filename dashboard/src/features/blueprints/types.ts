export interface BlueprintResource {
  id: string;
  name: string;
  type: string;
}

export interface BlueprintView {
  id: string;
  name: string;
  repo: string;
  branch: string;
  path: string;
  autoSync: boolean;
  manifest: string;
  status: string;
  lastSync: string | null;
  resources: BlueprintResource[] | null;
  createdAt: string | null;
  updatedAt: string | null;
}

export interface BlueprintSyncView {
  id: string;
  commitId: string;
  state: string;
  startedAt: string | null;
  completedAt: string | null;
}

export interface BlueprintValidationResult {
  valid: boolean;
  errors: string[];
}

export interface SyncBlueprintResult {
  blueprint: BlueprintView | null;
  services: Array<{ id: string; name: string } | null> | null;
  databases: string[] | null;
}
