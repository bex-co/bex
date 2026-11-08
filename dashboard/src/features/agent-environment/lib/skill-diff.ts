// Version history of a workspace skill (ADR099 D3): the server sends a unified
// line diff without file headers (`@@ -a,b +c,d @@`, then lines starting with
// " ", "-" or "+"). This turns it into rows with old/new line numbers.

export type SkillDiffRow =
  | { kind: "hunk"; text: string }
  | { kind: "context"; oldLine: number; newLine: number; text: string }
  | { kind: "remove"; oldLine: number; newLine: null; text: string }
  | { kind: "add"; oldLine: null; newLine: number; text: string };

const HUNK_HEADER = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/;

/**
 * Rows of a unified diff. Anything before the first hunk header, a
 * "\ No newline at end of file" marker and the empty tail after the final
 * newline are ignored; an absent or empty diff yields no rows.
 */
export function parseSkillDiff(
  diff: string | null | undefined,
): SkillDiffRow[] {
  if (!diff) return [];
  const lines = diff.split("\n");
  if (lines[lines.length - 1] === "") lines.pop();
  const rows: SkillDiffRow[] = [];
  let oldLine = 0;
  let newLine = 0;
  let inHunk = false;
  for (const line of lines) {
    const header = HUNK_HEADER.exec(line);
    if (header) {
      oldLine = Number(header[1]);
      newLine = Number(header[2]);
      inHunk = true;
      rows.push({ kind: "hunk", text: line });
      continue;
    }
    if (!inHunk || line.startsWith("\\")) continue;
    const text = line.slice(1);
    if (line.startsWith("+")) {
      rows.push({ kind: "add", oldLine: null, newLine: newLine++, text });
    } else if (line.startsWith("-")) {
      rows.push({ kind: "remove", oldLine: oldLine++, newLine: null, text });
    } else {
      rows.push({
        kind: "context",
        oldLine: oldLine++,
        newLine: newLine++,
        text,
      });
    }
  }
  return rows;
}

/** The lines of a whole file, for the numbered Content view. */
export function skillContentLines(
  content: string | null | undefined,
): string[] {
  if (!content) return [];
  const lines = content.split("\n");
  if (lines[lines.length - 1] === "") lines.pop();
  return lines;
}
