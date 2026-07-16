/**
 * One workspace environment group. List/detail reads intentionally carry only
 * variable keys and file names; sensitive values use dedicated reveal queries.
 */
export interface EnvGroupView {
  id: string;
  name: string;
  ownerId: string | null;
  createdAt: string | null;
  updatedAt: string | null;
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
}
