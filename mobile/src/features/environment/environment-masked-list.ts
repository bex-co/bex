export type MaskedEnvironmentVariable = Readonly<{
  id: string;
  key: string;
  revision: string;
}>;

export type MaskedEnvironmentList =
  | { valid: true; variables: MaskedEnvironmentVariable[] }
  | { valid: false; variables: [] };

type NullableMaskedItem = {
  id?: string | null;
  key?: string | null;
  revision?: string | null;
} | null;

/** Fails closed unless every key belongs to one coherent revision snapshot. */
export function parseMaskedEnvironmentList(
  items: readonly NullableMaskedItem[],
): MaskedEnvironmentList {
  const variables: MaskedEnvironmentVariable[] = [];
  const keys = new Set<string>();
  let revision: string | null = null;
  for (const item of items) {
    const key = item?.key?.trim();
    const itemRevision = item?.revision?.trim();
    if (!key || !itemRevision || keys.has(key)) {
      return { valid: false, variables: [] };
    }
    if (revision != null && itemRevision !== revision) {
      return { valid: false, variables: [] };
    }
    revision = itemRevision;
    keys.add(key);
    variables.push(
      Object.freeze({ id: item?.id?.trim() || key, key, revision }),
    );
  }
  return { valid: true, variables };
}
