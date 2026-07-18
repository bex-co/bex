import { describe, expect, it } from "vitest";
import {
  createEnvironmentDraft,
  environmentDraftPatch,
  isEnvironmentDraftDirty,
  isDraftValid,
  validateEnvironmentDraft,
} from "../environment-draft";

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
    expect(isEnvironmentDraftDirty(draft)).toBe(true);
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
});
