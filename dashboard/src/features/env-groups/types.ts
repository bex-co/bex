/**
 * One workspace environment group. List/detail reads intentionally carry only
 * variable keys and file names; sensitive values use dedicated reveal queries.
 */
export interface EnvGroupView {
  id: string;
  name: string;
  serviceLinks: string[];
  envVarKeys: string[];
  secretFileNames: string[];
}
