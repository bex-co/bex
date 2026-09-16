export const VALID_ENV_KEY = /^[A-Za-z_][A-Za-z0-9_]*$/;

/**
 * Names bex owns, mirroring `core.ReservedEnvKeys` in the backend. The operator
 * injects its own PORT and would silently drop a user's, so every write path
 * refuses the key (w2/m95 t003). Matching is exact and case-sensitive — `port`
 * and `APP_PORT` are ordinary application variables.
 *
 * The editor flags it inline so the refusal arrives as you type rather than on
 * Save; the server stays the authority.
 */
export const RESERVED_ENV_KEYS: readonly string[] = ["PORT"];

export function isReservedEnvKey(key: string): boolean {
  return RESERVED_ENV_KEYS.includes(key);
}
export const VALID_SECRET_FILE_NAME = /^[-._a-zA-Z0-9]+$/;
/**
 * The upload path's read guard: a file this large is never read into memory.
 * It is not the storage limit — that is MAX_ENVIRONMENT_MAP_BYTES in total.
 */
export const MAX_SECRET_FILE_BYTES = 1024 * 1024;

/**
 * bex-api's per-service (and per-environment-group) quota on each map:
 * `maxEnvKeys` / `maxSecretFiles` and `maxSecretMapBytes`, measured as every
 * name plus value in UTF-8 bytes (`lego/backend/internal/secrets/service.go`,
 * ADR066 #6). The draft checks what it can see; values the editor never
 * revealed are opaque here, so the server remains the authority (w1/m147).
 */
export const MAX_ENVIRONMENT_ENTRIES = 500;
export const MAX_ENVIRONMENT_MAP_BYTES = 512 * 1024;

export interface EnvDraftRow {
  id: string;
  originalKey: string | null;
  key: string;
  value: string | null;
  valueChanged: boolean;
  /** Server-side generation intent; the literal value remains absent. */
  generateValue?: boolean;
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
    generateValue?: boolean;
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
  env: Record<string, "invalid" | "duplicate" | "value" | "limit" | "reserved">;
  files: Record<string, "invalid" | "duplicate" | "content" | "limit">;
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
      generateValue: false,
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
  generated: (row: R) => boolean;
}

const ENV_LENS: RowLens<EnvDraftRow> = {
  original: (row) => row.originalKey,
  name: (row) => row.key,
  value: (row) => row.value,
  changed: (row) => row.valueChanged,
  generated: (row) => row.generateValue === true,
};

const FILE_LENS: RowLens<SecretFileDraftRow> = {
  original: (row) => row.originalName,
  name: (row) => row.name,
  value: (row) => row.content,
  changed: (row) => row.contentChanged,
  generated: () => false,
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
    if (
      lens.original(row) == null &&
      lens.value(row) == null &&
      !lens.generated(row)
    ) {
      errors[row.id] = missingValue;
    }
  }
  return errors;
}

const utf8 = new TextEncoder();

// flagOverLimit marks the rows that push a map past the quota, without
// replacing a more specific error. Only rows this draft adds or rewrites are
// flagged: they are what the save would grow the map by. An already-saved
// opaque row is never flagged, so a map that is over quota today can still be
// shrunk (deleting frees room, ADR066 #6).
function flagOverLimit<R extends { id: string; deleted: boolean }>(
  rows: readonly R[],
  lens: RowLens<R>,
  errors: Record<string, string>,
): void {
  const live = rows.filter((row) => !row.deleted);
  const knownBytes = live.reduce((total, row) => {
    const value = lens.value(row);
    return value == null
      ? total
      : total + utf8.encode(lens.name(row).trim() + value).length;
  }, 0);
  const tooMany = live.length > MAX_ENVIRONMENT_ENTRIES;
  const tooLarge = knownBytes > MAX_ENVIRONMENT_MAP_BYTES;
  if (!tooMany && !tooLarge) return;
  for (const row of live) {
    if (errors[row.id]) continue;
    const added = lens.original(row) == null;
    const rewritten = lens.value(row) != null && lens.changed(row);
    if ((tooMany && added) || (tooLarge && rewritten)) {
      errors[row.id] = "limit";
    }
  }
}

/**
 * Flag the rows that would *write* a reserved key. A row already stored under
 * that name and left alone is not flagged: the draft would send no operation
 * for it, so refusing would strand every unrelated edit on a service that
 * happens to hold a pre-rule PORT. Deleting one is always allowed.
 */
function flagReservedKeys(
  rows: readonly EnvDraftRow[],
  errors: DraftValidation["env"],
): void {
  for (const row of rows) {
    if (row.deleted) continue;
    const key = row.key.trim();
    if (!isReservedEnvKey(key)) continue;
    const writes =
      row.originalKey !== key || row.valueChanged || row.generateValue === true;
    if (writes) errors[row.id] = "reserved";
  }
}

export function validateEnvironmentDraft(
  draft: EnvironmentDraft,
): DraftValidation {
  const env: DraftValidation["env"] = validateRows(
    draft.envVars,
    ENV_LENS,
    (key) => VALID_ENV_KEY.test(key),
    "value",
  );
  const files: DraftValidation["files"] = validateRows(
    draft.secretFiles,
    FILE_LENS,
    isValidSecretFileName,
    "content",
  );
  flagReservedKeys(draft.envVars, env);
  flagOverLimit(draft.envVars, ENV_LENS, env);
  flagOverLimit(draft.secretFiles, FILE_LENS, files);
  return { env, files };
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
  generateValue?: boolean;
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
    const generateValue = lens.generated(row);
    if (row.deleted) {
      if (original) patch.push({ name: original, delete: true });
    } else if (!original) {
      patch.push({
        name,
        ...(generateValue ? { generateValue: true } : { value }),
      });
    } else if (name !== original) {
      // A rename with no new value moves the opaque value server-side; a rename
      // that also sets one cannot, so it becomes delete + create.
      if (!lens.changed(row) && !generateValue) {
        patch.push({ name, from: original });
      } else {
        patch.push(
          { name: original, delete: true },
          {
            name,
            ...(generateValue ? { generateValue: true } : { value }),
          },
        );
      }
    } else if (lens.changed(row) || generateValue) {
      patch.push({
        name,
        ...(generateValue ? { generateValue: true } : { value }),
      });
    }
  }
  return patch;
}

export function environmentDraftPatch(
  draft: EnvironmentDraft,
): EnvironmentPatchInput {
  return {
    envVars: patchRows(draft.envVars, ENV_LENS).map(
      ({ name, from, value, generateValue, delete: deleted }) => ({
        key: name,
        ...(from !== undefined && { fromKey: from }),
        ...(value !== undefined && { value }),
        ...(generateValue !== undefined && { generateValue }),
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
