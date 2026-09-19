/**
 * One workspace environment group. List/detail reads intentionally carry only
 * variable keys and file names; sensitive values use dedicated reveal queries.
 */
export interface EnvGroupView {
  id: string;
  name: string;
  ownerId: string | null;
  environmentId: string | null;
  createdAt: string | null;
  updatedAt: string | null;
  revision: string | null;
  /** Non-null when the group is busy or needs repair after an interrupted save. */
  availability: "busy" | "repair_required" | null;
  serviceLinks: string[];
  envVarKeys: string[];
  secretFileNames: string[];
}

export interface CreateEnvGroupInput {
  name: string;
  envVars: Array<{
    key: string;
    value?: string;
    generateValue?: boolean;
  }>;
  secretFiles: Array<{ name: string; content: string }>;
  serviceIds: string[];
  /**
   * The Environment the group is minted in; `null`/omitted is workspace scope
   * (the pre-w4/m111 behavior, and still the default). Every linked service
   * must live in the same scope or bex-api refuses the create.
   */
  environmentId?: string | null;
}
