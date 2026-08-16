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

/** True for a row added in this draft — one with no counterpart on the server. */
export function isNewDraftRow(row: EnvDraftRow | SecretFileDraftRow): boolean {
  return ("originalKey" in row ? row.originalKey : row.originalName) == null;
}

/** The mask standing in for an unrevealed value, shared by every row renderer. */
export const MASKED_VALUE = "••••••••••••";

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

// Env vars and secret files are the same row algorithm under two field names
// (key/value/valueChanged vs name/content/contentChanged). A lens names the
// four members once per kind so validation and patch derivation are written
// once — the two spellings had drifted apart before only by luck.
interface RowLens<R> {
  original: (row: R) => string | null;
  name: (row: R) => string;
  value: (row: R) => string | null;
  changed: (row: R) => boolean;
}

const ENV_LENS: RowLens<EnvDraftRow> = {
  original: (row) => row.originalKey,
  name: (row) => row.key,
  value: (row) => row.value,
  changed: (row) => row.valueChanged,
};

const FILE_LENS: RowLens<SecretFileDraftRow> = {
  original: (row) => row.originalName,
  name: (row) => row.name,
  value: (row) => row.content,
  changed: (row) => row.contentChanged,
};

function validateRows<
  R extends { id: string; deleted: boolean },
  E extends string,
>(
  rows: readonly R[],
  lens: RowLens<R>,
  isValidName: (name: string) => boolean,
  missingValue: E,
): Record<string, E | "invalid" | "duplicate"> {
  const errors: Record<string, E | "invalid" | "duplicate"> = {};
  const seen = new Map<string, string>();
  for (const row of rows.filter((row) => !row.deleted)) {
    const name = lens.name(row).trim();
    if (!isValidName(name)) errors[row.id] = "invalid";
    const prior = seen.get(name);
    if (prior) {
      errors[prior] = "duplicate";
      errors[row.id] = "duplicate";
    } else seen.set(name, row.id);
    if (lens.original(row) == null && lens.value(row) == null) {
      errors[row.id] = missingValue;
    }
  }
  return errors;
}

export function validateEnvironmentDraft(
  draft: EnvironmentDraft,
): DraftValidation {
  return {
    env: validateRows(
      draft.envVars,
      ENV_LENS,
      (key) => VALID_ENV_KEY.test(key),
      "value",
    ),
    files: validateRows(
      draft.secretFiles,
      FILE_LENS,
      isValidSecretFileName,
      "content",
    ),
  };
}

export function isDraftValid(validation: DraftValidation): boolean {
  return (
    Object.keys(validation.env).length === 0 &&
    Object.keys(validation.files).length === 0
  );
}

// One patch operation in the neutral vocabulary the two kinds share; each call
// site below renames the two members to its own wire keys.
interface RowPatch {
  name: string;
  from?: string;
  value?: string;
  delete?: boolean;
}

function patchRows<R extends { deleted: boolean }>(
  rows: readonly R[],
  lens: RowLens<R>,
): RowPatch[] {
  const patch: RowPatch[] = [];
  for (const row of rows) {
    const original = lens.original(row);
    const name = lens.name(row).trim();
    const value = lens.value(row) ?? "";
    if (row.deleted) {
      if (original) patch.push({ name: original, delete: true });
    } else if (!original) {
      patch.push({ name, value });
    } else if (name !== original) {
      // A rename with no new value moves the opaque value server-side; a rename
      // that also sets one cannot, so it becomes delete + create.
      if (!lens.changed(row)) patch.push({ name, from: original });
      else patch.push({ name: original, delete: true }, { name, value });
    } else if (lens.changed(row)) {
      patch.push({ name, value });
    }
  }
  return patch;
}

export function environmentDraftPatch(
  draft: EnvironmentDraft,
): EnvironmentPatchInput {
  return {
    envVars: patchRows(draft.envVars, ENV_LENS).map(
      ({ name, from, value, delete: deleted }) => ({
        key: name,
        ...(from !== undefined && { fromKey: from }),
        ...(value !== undefined && { value }),
        ...(deleted !== undefined && { delete: deleted }),
      }),
    ),
    secretFiles: patchRows(draft.secretFiles, FILE_LENS).map(
      ({ name, from, value, delete: deleted }) => ({
        name,
        ...(from !== undefined && { fromName: from }),
        ...(value !== undefined && { content: value }),
        ...(deleted !== undefined && { delete: deleted }),
      }),
    ),
  };
}

export function isEnvironmentDraftDirty(draft: EnvironmentDraft): boolean {
  const patch = environmentDraftPatch(draft);
  return patch.envVars.length > 0 || patch.secretFiles.length > 0;
}

export function isValidSecretFileName(name: string): boolean {
  return VALID_SECRET_FILE_NAME.test(name) && name !== "." && name !== "..";
}
