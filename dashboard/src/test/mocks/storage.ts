/**
 * A real in-memory Storage. src/test/setup.ts installs a no-op localStorage,
 * so a test where persistence is the point stubs this in:
 * `vi.stubGlobal("localStorage", memoryStorage())`.
 */
export function memoryStorage(): Storage {
  const items = new Map<string, string>();
  return {
    get length() {
      return items.size;
    },
    clear: () => items.clear(),
    getItem: (k) => items.get(k) ?? null,
    key: (i) => [...items.keys()][i] ?? null,
    removeItem: (k) => void items.delete(k),
    setItem: (k, v) => void items.set(k, String(v)),
  };
}
