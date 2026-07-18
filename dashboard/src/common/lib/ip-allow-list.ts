export interface IPAllowListEntryDraft {
  cidrBlock: string;
  description: string;
}

export function ipAllowListEntryKey(entries: IPAllowListEntryDraft[]) {
  return JSON.stringify(entries);
}
