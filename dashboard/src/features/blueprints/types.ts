export interface BlueprintView {
  id: string;
  name: string;
  repo: string;
  branch: string;
  manifest: string;
  status: string;
  createdAt: string | null;
  updatedAt: string | null;
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
