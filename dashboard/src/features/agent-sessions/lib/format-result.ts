/** Pretty-prints JSON text; anything unparseable renders as received. */
export function formatResultJson(text: string): string {
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return text;
  }
}
