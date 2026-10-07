/**
 * record's own value for key, never one inherited from Object.prototype (a
 * server-sent code "toString" must not resolve to a function).
 */
export function ownValue<V>(
  record: Readonly<Record<string, V>>,
  key: string | null | undefined,
): V | undefined {
  return key && Object.hasOwn(record, key) ? record[key] : undefined;
}
