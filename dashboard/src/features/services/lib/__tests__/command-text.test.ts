import { describe, expect, it } from "vitest";
import { commandText } from "../command-text";

// (w8/m68)
describe("commandText", () => {
  it.each([
    ["an escaped space", "printf '%s' tail\\ "],
    ["an escaped tab", "printf '%s' tail\\\t"],
    ["surrounding padding", "  npm run migrate  "],
    ["inner whitespace", "sh -c 'echo  a\tb'"],
  ])("keeps a nonblank command with %s byte for byte", (_, command) => {
    expect(commandText(command)).toBe(command);
  });

  it.each([
    ["nothing", ""],
    ["spaces", "   "],
    ["a tab", "\t"],
    ["mixed blanks", " \t \u00a0 "],
  ])("collapses a draft of %s to the empty string", (_, draft) => {
    expect(commandText(draft)).toBe("");
  });
});
