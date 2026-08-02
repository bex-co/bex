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

export interface BlueprintPreviewPlan {
  services: string[] | null;
  databases: string[] | null;
  keyValue: string[] | null;
  envGroups: string[] | null;
  totalActions: number | null;
}

export interface BlueprintPreviewValidation {
  valid: boolean | null;
  errors: string[] | null;
  plan: BlueprintPreviewPlan | null;
}

export interface BlueprintPreviewResult {
  found: boolean | null;
  commitId: string | null;
  error: string | null;
  validation: BlueprintPreviewValidation | null;
}

export interface SyncBlueprintResult {
  blueprint: BlueprintView | null;
  services: Array<{ id: string; name: string } | null> | null;
  databases: string[] | null;
}
