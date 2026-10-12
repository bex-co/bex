import { describe, expect, it } from "vitest";
import {
  MAX_ENVIRONMENT_ENTRIES,
  MAX_ENVIRONMENT_MAP_BYTES,
  createEnvironmentDraft,
  environmentDraftPatch,
  isDraftValid,
  reidentifyDraft,
  validateEnvironmentDraft,
  VALID_ENV_KEY,
  isValidSecretFileName,
  type EnvironmentDraft,
} from "../environment-draft";
import { readRepoVectors, repeated } from "@/test/repo-vectors";

function addFile(draft: EnvironmentDraft, name: string, content: string) {
  draft.secretFiles.push({
    id: `new:${name}`,
    originalName: null,
    name,
    content,
    contentChanged: true,
    deleted: false,
  });
}

function addVar(draft: EnvironmentDraft, key: string, value: string) {
  draft.envVars.push({
    id: `new:${key}`,
    originalKey: null,
    key,
    value,
    valueChanged: true,
    deleted: false,
  });
}

const names = (prefix: string, n: number) =>
  Array.from({ length: n }, (_, i) => `${prefix}${i}`);

describe("environment draft", () => {
  it("preserves unchanged opaque values and models opaque renames", () => {
    const draft = createEnvironmentDraft(["KEEP", "RENAME"], ["keep.pem"]);
    draft.envVars[1].key = "RENAMED";
    draft.secretFiles[0].name = "renamed.pem";
    expect(environmentDraftPatch(draft)).toEqual({
      envVars: [{ key: "RENAMED", fromKey: "RENAME" }],
      secretFiles: [{ name: "renamed.pem", fromName: "keep.pem" }],
    });
  });

  it("derives mixed add/update/delete operations without unchanged values", () => {
    const draft = createEnvironmentDraft(["KEEP", "DROP"], ["drop.pem"]);
    draft.envVars[0].value = "replacement";
    draft.envVars[0].valueChanged = true;
    draft.envVars[1].deleted = true;
    draft.envVars.push({
      id: "new",
      originalKey: null,
      key: "ADDED",
      value: "new",
      valueChanged: true,
      deleted: false,
    });
    draft.secretFiles[0].deleted = true;
    expect(environmentDraftPatch(draft)).toEqual({
      envVars: [
        { key: "DROP", delete: true },
        { key: "KEEP", value: "replacement" },
        { key: "ADDED", value: "new" },
      ],
      secretFiles: [{ name: "drop.pem", delete: true }],
    });
  });

  it.each([
    {
      description: "renames a revealed value without rewriting it",
      changes: { key: " AFTER ", value: "revealed" },
      expected: [{ key: "AFTER", fromKey: "BEFORE" }],
    },
    {
      description: "deletes the old key before renaming with a replacement",
      changes: { key: "AFTER", value: "replacement", valueChanged: true },
      expected: [
        { key: "BEFORE", delete: true },
        { key: "AFTER", value: "replacement" },
      ],
    },
    {
      description: "keeps an explicit empty replacement on rename",
      changes: { key: "AFTER", valueChanged: true },
      expected: [
        { key: "BEFORE", delete: true },
        { key: "AFTER", value: "" },
      ],
    },
    {
      description: "regenerates under the same key without sending its value",
      changes: { value: "revealed", generateValue: true },
      expected: [{ key: "BEFORE", generateValue: true }],
    },
    {
      description: "deletes the old key before renaming with generation",
      changes: { key: "AFTER", value: "replacement", generateValue: true },
      expected: [
        { key: "BEFORE", delete: true },
        { key: "AFTER", generateValue: true },
      ],
    },
    {
      description: "deletes the original key despite later draft edits",
      changes: { key: "AFTER", generateValue: true, deleted: true },
      expected: [{ key: "BEFORE", delete: true }],
    },
    {
      description: "omits a new row deleted before saving",
      changes: { originalKey: null, deleted: true },
      expected: [],
    },
    {
      description: "creates a generated row without sending its value",
      changes: { originalKey: null, value: "ignored", generateValue: true },
      expected: [{ key: "BEFORE", generateValue: true }],
    },
    {
      description: "treats an empty original key as a new row",
      changes: { originalKey: "", value: "new" },
      expected: [{ key: "BEFORE", value: "new" }],
    },
  ])("$description", ({ changes, expected }) => {
    const draft = createEnvironmentDraft(["BEFORE"], []);
    Object.assign(draft.envVars[0], changes);
    expect(environmentDraftPatch(draft)).toEqual({
      envVars: expected,
      secretFiles: [],
    });
  });

  it("orders secret-file removals, then renames, then writes", () => {
    const draft = createEnvironmentDraft([], ["opaque.pem", "replace.pem"]);
    draft.secretFiles[0].name = " moved.pem ";
    draft.secretFiles[0].content = "revealed";
    draft.secretFiles[1].name = "renamed.pem";
    draft.secretFiles[1].contentChanged = true;
    addFile(draft, "removed.pem", "temporary");
    draft.secretFiles[2].deleted = true;
    addFile(draft, "added.pem", "new");
    expect(environmentDraftPatch(draft)).toEqual({
      envVars: [],
      secretFiles: [
        { name: "replace.pem", delete: true },
        { name: "moved.pem", fromName: "opaque.pem" },
        { name: "renamed.pem", content: "" },
        { name: "added.pem", content: "new" },
      ],
    });
  });

  // w4/m195: deleting a stored name and renaming another opaque value into it
  // must free the name first, whichever row the store happens to list first.
  it.each([
    { order: "source before destination", stored: ["NEW", "RIGHT"] },
    { order: "destination before source", stored: ["RIGHT", "NEW"] },
  ])(
    "deletes a destination before an opaque rename into it ($order)",
    ({ stored }) => {
      const files = stored.map((name) => name.toLowerCase());
      const draft = createEnvironmentDraft(stored, files);
      const at = stored.indexOf("RIGHT");
      const from = stored.indexOf("NEW");
      draft.envVars[at].deleted = true;
      draft.envVars[from].key = "RIGHT";
      draft.secretFiles[at].deleted = true;
      draft.secretFiles[from].name = "right";
      expect(isDraftValid(validateEnvironmentDraft(draft))).toBe(true);
      expect(environmentDraftPatch(draft)).toEqual({
        envVars: [
          { key: "RIGHT", delete: true },
          { key: "RIGHT", fromKey: "NEW" },
        ],
        secretFiles: [
          { name: "right", delete: true },
          { name: "right", fromName: "new" },
        ],
      });
    },
  );

  // w4/243: Delete a stored row, then Add one under the same name. The final
  // draft names it once, so the save is one write — the API refuses a write
  // after an explicit delete of its name, which stranded every such draft.
  it.each([
    {
      kind: "generated secret",
      replace: (draft: EnvironmentDraft) => {
        addVar(draft, "QA_A118_RENAMED", "");
        Object.assign(draft.envVars.at(-1)!, {
          value: null,
          generateValue: true,
        });
      },
      envVars: [{ key: "QA_A118_RENAMED", generateValue: true }],
    },
    {
      kind: "literal",
      replace: (draft: EnvironmentDraft) =>
        addVar(draft, "QA_A118_RENAMED", "qa-a118-readd-literal"),
      envVars: [{ key: "QA_A118_RENAMED", value: "qa-a118-readd-literal" }],
    },
    {
      kind: "empty literal",
      replace: (draft: EnvironmentDraft) =>
        addVar(draft, "QA_A118_RENAMED", ""),
      envVars: [{ key: "QA_A118_RENAMED", value: "" }],
    },
  ])(
    "re-adding a deleted variable sends one write ($kind)",
    ({ replace, envVars }) => {
      const draft = createEnvironmentDraft(["QA_A118_RENAMED", "KEEP"], []);
      draft.envVars[0].deleted = true;
      replace(draft);
      expect(isDraftValid(validateEnvironmentDraft(draft))).toBe(true);
      expect(environmentDraftPatch(draft)).toEqual({
        envVars,
        secretFiles: [],
      });
    },
  );

  it.each([
    { kind: "empty", content: "" },
    { kind: "nonempty", content: "qa-a165-file-v2\nnew β\n" },
  ])(
    "re-adding a deleted secret file sends one write ($kind)",
    ({ content }) => {
      const draft = createEnvironmentDraft([], ["qa-a165-renamed.txt"]);
      draft.secretFiles[0].deleted = true;
      addFile(draft, "qa-a165-renamed.txt", content);
      expect(isDraftValid(validateEnvironmentDraft(draft))).toBe(true);
      expect(environmentDraftPatch(draft)).toEqual({
        envVars: [],
        secretFiles: [{ name: "qa-a165-renamed.txt", content }],
      });
    },
  );

  it("keeps unrelated deletions and renames while coalescing re-added names", () => {
    const draft = createEnvironmentDraft(
      ["A", "B", "GONE", "MOVE"],
      ["a.pem", "b.pem", "gone.pem"],
    );
    for (const row of [...draft.envVars.slice(0, 3), ...draft.secretFiles]) {
      row.deleted = true;
    }
    draft.envVars[3].key = "MOVED";
    addVar(draft, "B", "b2");
    addVar(draft, "A", "a2");
    addFile(draft, "a.pem", "a2");
    addFile(draft, "b.pem", "");
    expect(isDraftValid(validateEnvironmentDraft(draft))).toBe(true);
    expect(environmentDraftPatch(draft)).toEqual({
      envVars: [
        { key: "GONE", delete: true },
        { key: "MOVED", fromKey: "MOVE" },
        { key: "B", value: "b2" },
        { key: "A", value: "a2" },
      ],
      secretFiles: [
        { name: "gone.pem", delete: true },
        { name: "a.pem", content: "a2" },
        { name: "b.pem", content: "" },
      ],
    });
  });

  // A renamed row whose value changed still removes its old name — unless a
  // new row writes that old name, which then replaces it in place.
  it("drops a replaced row's old-name removal only when that name is rewritten", () => {
    const draft = createEnvironmentDraft(["OLD"], []);
    draft.envVars[0].key = "NEW";
    draft.envVars[0].value = "v2";
    draft.envVars[0].valueChanged = true;
    expect(environmentDraftPatch(draft).envVars).toEqual([
      { key: "OLD", delete: true },
      { key: "NEW", value: "v2" },
    ]);
    addVar(draft, "OLD", "fresh");
    expect(environmentDraftPatch(draft).envVars).toEqual([
      { key: "NEW", value: "v2" },
      { key: "OLD", value: "fresh" },
    ]);
  });

  // w4/234: moving a stored value away and uploading a new one at its old
  // name sends the move before the write, even when the new row is listed
  // first, so the name is vacant when the write lands.
  it.each([
    { order: "upload after the moved row", first: false },
    { order: "upload before the moved row", first: true },
  ])("moves a value away before writing its old name ($order)", ({ first }) => {
    const draft = createEnvironmentDraft(["OLD"], ["original.txt"]);
    draft.envVars[0].key = "MOVED";
    draft.secretFiles[0].name = "renamed.txt";
    addVar(draft, "OLD", "fresh");
    addFile(draft, "original.txt", "replacement\r\n");
    addFile(draft, "empty.txt", "");
    if (first) {
      draft.envVars.reverse();
      draft.secretFiles.unshift(draft.secretFiles.splice(1, 1)[0]);
    }
    expect(isDraftValid(validateEnvironmentDraft(draft))).toBe(true);
    expect(environmentDraftPatch(draft)).toEqual({
      envVars: [
        { key: "MOVED", fromKey: "OLD" },
        { key: "OLD", value: "fresh" },
      ],
      secretFiles: [
        { name: "renamed.txt", fromName: "original.txt" },
        { name: "original.txt", content: "replacement\r\n" },
        { name: "empty.txt", content: "" },
      ],
    });
  });

  it("blocks invalid and duplicate names", () => {
    const draft = createEnvironmentDraft(["ONE", "TWO"], []);
    draft.envVars[0].key = "DUP";
    draft.envVars[1].key = "DUP";
    const validation = validateEnvironmentDraft(draft);
    expect(isDraftValid(validation)).toBe(false);
    expect(validation.env).toEqual({
      "env:ONE": "duplicate",
      "env:TWO": "duplicate",
    });
  });

  // w1/m147: bex-api caps each map at 500 entries and 512 KiB of names plus
  // values. The editor used to allow any number of files up to 1 MiB each, so
  // the 614,400-byte file from the milestone's evidence reached the server.
  describe("quota", () => {
    it("allows a new file whose name and content are exactly 512 KiB", () => {
      const draft = createEnvironmentDraft([], []);
      addFile(draft, "a.pem", "x".repeat(MAX_ENVIRONMENT_MAP_BYTES - 5));
      expect(isDraftValid(validateEnvironmentDraft(draft))).toBe(true);
    });

    it("flags a new file that takes the known total one byte past 512 KiB", () => {
      const draft = createEnvironmentDraft([], []);
      addFile(draft, "a.pem", "x".repeat(MAX_ENVIRONMENT_MAP_BYTES - 4));
      const validation = validateEnvironmentDraft(draft);
      expect(validation.files).toEqual({ "new:a.pem": "limit" });
      expect(isDraftValid(validation)).toBe(false);
    });

    it("flags the milestone's 614,400-byte upload", () => {
      const draft = createEnvironmentDraft([], ["existing.pem"]);
      addFile(draft, "big.bin", "x".repeat(614_400));
      expect(validateEnvironmentDraft(draft).files).toEqual({
        "new:big.bin": "limit",
      });
    });

    it("measures UTF-8 bytes, not characters", () => {
      const draft = createEnvironmentDraft([], []);
      // "é" is two bytes: half as many characters already fill the map.
      addFile(draft, "a", "é".repeat(MAX_ENVIRONMENT_MAP_BYTES / 2));
      expect(validateEnvironmentDraft(draft).files).toEqual({
        "new:a": "limit",
      });
    });

    it("allows the 500th file and flags the 501st", () => {
      const draft = createEnvironmentDraft(
        [],
        names("f", MAX_ENVIRONMENT_ENTRIES - 1),
      );
      addFile(draft, "last.pem", "x");
      expect(isDraftValid(validateEnvironmentDraft(draft))).toBe(true);

      addFile(draft, "over.pem", "x");
      const validation = validateEnvironmentDraft(draft);
      expect(validation.files["new:over.pem"]).toBe("limit");
      expect(isDraftValid(validation)).toBe(false);
    });

    it("flags the 501st environment variable and oversized values", () => {
      const counted = createEnvironmentDraft(
        names("KEY_", MAX_ENVIRONMENT_ENTRIES),
        [],
      );
      addVar(counted, "ONE_MORE", "x");
      expect(validateEnvironmentDraft(counted).env).toEqual({
        "new:ONE_MORE": "limit",
      });

      const sized = createEnvironmentDraft([], []);
      addVar(sized, "BIG", "x".repeat(MAX_ENVIRONMENT_MAP_BYTES));
      expect(validateEnvironmentDraft(sized).env).toEqual({
        "new:BIG": "limit",
      });
    });

    it("never flags saved rows, so an over-quota map can still be shrunk", () => {
      const draft = createEnvironmentDraft(
        [],
        names("f", MAX_ENVIRONMENT_ENTRIES + 3),
      );
      draft.secretFiles[0].deleted = true;
      expect(isDraftValid(validateEnvironmentDraft(draft))).toBe(true);
    });

    it("keeps a more specific error ahead of the limit", () => {
      const draft = createEnvironmentDraft(
        [],
        names("f", MAX_ENVIRONMENT_ENTRIES),
      );
      addFile(draft, "bad name", "x");
      expect(validateEnvironmentDraft(draft).files).toEqual({
        "new:bad name": "invalid",
      });
    });
  });
});

// w4/m161: an environment-group draft's base revision is state, not identity.
// Re-keying a restored draft must keep it — dropping it would leave the save
// with no base, and the old code borrowed whatever revision the page polled.
describe("reidentifyDraft", () => {
  it("re-keys rows but keeps the draft's base revision", () => {
    const draft: EnvironmentDraft = {
      ...createEnvironmentDraft(["TOKEN"], ["cert.pem"]),
      baseRevision: "egr1_base",
    };
    let next = 0;
    const restored = reidentifyDraft(draft, () => next++);
    expect(restored.baseRevision).toBe("egr1_base");
    expect(restored.envVars.map((row) => row.id)).toEqual(["restored-env:0"]);
    expect(restored.secretFiles.map((row) => row.id)).toEqual([
      "restored-file:1",
    ]);
  });

  it("does not invent a base for a draft stored without one", () => {
    const restored = reidentifyDraft(
      createEnvironmentDraft(["TOKEN"], []),
      () => 0,
    );
    expect(restored).not.toHaveProperty("baseRevision");
  });
});

// The one table of environment variable and secret-file names. bex-api's
// ValidEnvKey and ValidSecretFileName are tested against the same file
// (lego/backend/internal/core/environmentpatch_test.go), and its answers, which
// are Kubernetes' own Secret-key rule, are the source of truth (w5/m118).
const CONFIG_KEY_VECTORS = readRepoVectors<{
  key: string;
  repeat?: number;
  envVar: boolean;
  secretFile: boolean;
}>("lego/backend/internal/core/testdata/config-key-vectors.json");

describe("the shared config-key vectors", () => {
  it.each(
    CONFIG_KEY_VECTORS.map(
      (v) => [repeated(v.key, v.repeat), v.envVar, v.secretFile] as const,
    ),
  )("%j is an env key: %s, a secret file name: %s", (key, envVar, file) => {
    expect(VALID_ENV_KEY.test(key)).toBe(envVar);
    expect(isValidSecretFileName(key)).toBe(file);
  });
});

// w4/m168: names cap at 253 characters, Kubernetes' Secret-key limit and
// bex-api's — a longer name used to be stored and then fail the projection.
describe("name length cap", () => {
  it("blocks Save on a 254-character file name and keeps 253", () => {
    const tooLong = createEnvironmentDraft([], []);
    addFile(tooLong, "f".repeat(254), "x");
    expect(isDraftValid(validateEnvironmentDraft(tooLong))).toBe(false);

    const atLimit = createEnvironmentDraft([], []);
    addFile(atLimit, "f".repeat(253), "x");
    expect(isDraftValid(validateEnvironmentDraft(atLimit))).toBe(true);
  });
});
