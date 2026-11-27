import { describe, expect, it } from "vitest";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { blueprintSettingsRefusal } from "@/features/blueprints/lib/settings-refusal";
import { translator } from "@/test/translator";

const t = translator("en");

function refused(code: string, message: string) {
  return new CombinedGraphQLErrors({
    data: null,
    errors: [{ message, extensions: { code } }],
  });
}

// Render's `sL` and its name row (w3/m679): the sentence a Settings row shows
// for each refusal bex-api answers.
describe("blueprintSettingsRefusal", () => {
  const source = { path: "infra/render.yaml", branch: "staging" };

  it.each([
    [
      refused("BLUEPRINT_NAME_TAKEN", 'a Blueprint named "x" already exists'),
      "Name already in use",
    ],
    [
      refused("branch_not_found", 'Branch "staging" does not exist.'),
      "Branch not found in this repository. Choose another branch.",
    ],
    [
      refused(
        "file_not_found",
        'Blueprint file "infra/render.yaml" not found.',
      ),
      'Blueprint file "infra/render.yaml" not found on "staging" branch. Please check that the file exists on this branch.',
    ],
    [
      refused("BLUEPRINT_AUTOSYNC_UNREACHABLE", "pushes cannot reach bex"),
      "Pushes cannot reach bex",
    ],
    [
      refused("INTERNAL", "internal error"),
      "Failed to update Blueprint. Please try again.",
    ],
    [
      new Error("Failed to fetch"),
      "Failed to update Blueprint. Please try again.",
    ],
  ])("words %o as Render does", (error, sentence) => {
    expect(blueprintSettingsRefusal(error, source, t)).toBe(sentence);
  });

  it("names render.yaml for a Blueprint with no path of its own", () => {
    expect(
      blueprintSettingsRefusal(
        refused("file_not_found", "not found"),
        { path: "", branch: "main" },
        t,
      ),
    ).toBe(
      'Blueprint file "render.yaml" not found on "main" branch. Please check that the file exists on this branch.',
    );
  });
});
