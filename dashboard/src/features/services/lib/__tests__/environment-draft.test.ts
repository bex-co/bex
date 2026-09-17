import { describe, expect, it } from "vitest";
import {
  MAX_ENVIRONMENT_ENTRIES,
  MAX_ENVIRONMENT_MAP_BYTES,
  createEnvironmentDraft,
  environmentDraftPatch,
  isDraftValid,
  validateEnvironmentDraft,
  type EnvironmentDraft,
} from "../environment-draft";

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
        { key: "KEEP", value: "replacement" },
        { key: "DROP", delete: true },
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

  it("preserves secret-file rename, replacement, and draft-deletion order", () => {
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
        { name: "moved.pem", fromName: "opaque.pem" },
        { name: "replace.pem", delete: true },
        { name: "renamed.pem", content: "" },
        { name: "added.pem", content: "new" },
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
