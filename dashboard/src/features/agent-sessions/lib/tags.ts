/**
 * Splits the tag editor's text into tags (w7/m296): commas or whitespace
 * separate them, empty pieces are dropped. The server validates the rest.
 */
export function parseTagInput(text: string): string[] {
  return text.split(/[\s,]+/).filter((tag) => tag !== "");
}
