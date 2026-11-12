/** An automation's minted id (aut- plus a 20-character xid), not its name. */
export function isAutomationId(value: string): boolean {
  return /^aut-[0-9a-v]{20}$/.test(value);
}
