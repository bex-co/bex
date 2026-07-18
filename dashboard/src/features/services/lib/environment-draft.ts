export const VALID_ENV_KEY = /^[A-Za-z_][A-Za-z0-9_]*$/;
export const VALID_SECRET_FILE_NAME = /^[-._a-zA-Z0-9]+$/;
export const MAX_SECRET_FILE_BYTES = 1024 * 1024;

export interface EnvDraftRow {
  id: string;
  originalKey: string | null;
  key: string;
  value: string | null;
  valueChanged: boolean;
  deleted: boolean;
}

export interface SecretFileDraftRow {
  id: string;
  originalName: string | null;
  name: string;
  content: string | null;
  contentChanged: boolean;
  deleted: boolean;
}

export interface EnvironmentDraft {
  envVars: EnvDraftRow[];
  secretFiles: SecretFileDraftRow[];
}

export interface EnvironmentPatchInput {
  envVars: Array<{
    key: string;
    fromKey?: string;
    value?: string;
    delete?: boolean;
  }>;
  secretFiles: Array<{
    name: string;
    fromName?: string;
    content?: string;
    delete?: boolean;
  }>;
}

export interface DraftValidation {
  env: Record<string, "invalid" | "duplicate" | "value">;
  files: Record<string, "invalid" | "duplicate" | "content">;
}

export function createEnvironmentDraft(
  envKeys: readonly string[],
  fileNames: readonly string[],
): EnvironmentDraft {
  return {
    envVars: envKeys.map((key) => ({
      id: `env:${key}`,
      originalKey: key,
      key,
      value: null,
      valueChanged: false,
      deleted: false,
    })),
    secretFiles: fileNames.map((name) => ({
      id: `file:${name}`,
      originalName: name,
      name,
      content: null,
      contentChanged: false,
      deleted: false,
    })),
  };
}

export function validateEnvironmentDraft(
  draft: EnvironmentDraft,
): DraftValidation {
  const validation: DraftValidation = { env: {}, files: {} };
  const envNames = new Map<string, string>();
  for (const row of draft.envVars.filter((row) => !row.deleted)) {
    const key = row.key.trim();
    if (!VALID_ENV_KEY.test(key)) validation.env[row.id] = "invalid";
    const prior = envNames.get(key);
    if (prior) {
      validation.env[prior] = "duplicate";
      validation.env[row.id] = "duplicate";
    } else envNames.set(key, row.id);
    if (row.originalKey == null && row.value == null) {
      validation.env[row.id] = "value";
    }
  }
  const fileNames = new Map<string, string>();
  for (const row of draft.secretFiles.filter((row) => !row.deleted)) {
    const name = row.name.trim();
    if (!isValidSecretFileName(name)) validation.files[row.id] = "invalid";
    const prior = fileNames.get(name);
    if (prior) {
      validation.files[prior] = "duplicate";
      validation.files[row.id] = "duplicate";
    } else fileNames.set(name, row.id);
    if (row.originalName == null && row.content == null) {
      validation.files[row.id] = "content";
    }
  }
  return validation;
}

export function isDraftValid(validation: DraftValidation): boolean {
  return (
    Object.keys(validation.env).length === 0 &&
    Object.keys(validation.files).length === 0
  );
}

export function environmentDraftPatch(
  draft: EnvironmentDraft,
): EnvironmentPatchInput {
  const envVars: EnvironmentPatchInput["envVars"] = [];
  for (const row of draft.envVars) {
    const key = row.key.trim();
    if (row.deleted) {
      if (row.originalKey) envVars.push({ key: row.originalKey, delete: true });
      continue;
    }
    if (!row.originalKey) {
      envVars.push({ key, value: row.value ?? "" });
      continue;
    }
    if (key !== row.originalKey) {
      if (!row.valueChanged) {
        envVars.push({ key, fromKey: row.originalKey });
      } else {
        envVars.push({ key: row.originalKey, delete: true });
        envVars.push({ key, value: row.value ?? "" });
      }
    } else if (row.valueChanged) {
      envVars.push({ key, value: row.value ?? "" });
    }
  }

  const secretFiles: EnvironmentPatchInput["secretFiles"] = [];
  for (const row of draft.secretFiles) {
    const name = row.name.trim();
    if (row.deleted) {
      if (row.originalName) {
        secretFiles.push({ name: row.originalName, delete: true });
      }
      continue;
    }
    if (!row.originalName) {
      secretFiles.push({ name, content: row.content ?? "" });
      continue;
    }
    if (name !== row.originalName) {
      if (!row.contentChanged) {
        secretFiles.push({ name, fromName: row.originalName });
      } else {
        secretFiles.push({ name: row.originalName, delete: true });
        secretFiles.push({ name, content: row.content ?? "" });
      }
    } else if (row.contentChanged) {
      secretFiles.push({ name, content: row.content ?? "" });
    }
  }
  return { envVars, secretFiles };
}

export function isEnvironmentDraftDirty(draft: EnvironmentDraft): boolean {
  const patch = environmentDraftPatch(draft);
  return patch.envVars.length > 0 || patch.secretFiles.length > 0;
}

export function isValidSecretFileName(name: string): boolean {
  return VALID_SECRET_FILE_NAME.test(name) && name !== "." && name !== "..";
}
