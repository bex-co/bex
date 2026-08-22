// Frequency-ranked repo quick-select for the composer. The ranking/bump math is
// pure and unit-tested; AsyncStorage is a thin per-workspace wrapper. Repo names
// are not secrets (the picker already lists them), so plain storage keyed by the
// workspace id is fine — an old value is unreachable once the workspace changes.
import AsyncStorage from "@react-native-async-storage/async-storage";

export interface RepoUse {
  count: number;
  lastUsed: number;
}

export type RepoFrequency = Record<string, RepoUse>;

function storageKey(ownerId: string): string {
  return `agent-repo-frequency:${ownerId}`;
}

// Pure: increment a repo's use count and stamp recency, returning a new map.
// Whitespace-only names are ignored so a blank selection never earns a pill.
export function bumpRepo(
  freq: RepoFrequency,
  repo: string,
  now: number,
): RepoFrequency {
  const name = repo.trim();
  if (name === "") return freq;
  const prev = freq[name];
  return { ...freq, [name]: { count: (prev?.count ?? 0) + 1, lastUsed: now } };
}

// Pure: the top `limit` repos to surface as quick-select pills. Repos with
// recorded use rank first by count, then by recency; any remaining slots are
// filled from `available` in list order so the row is useful before any history
// exists. Only repos still present in `available` are ever returned.
export function rankRepos(
  freq: RepoFrequency,
  available: string[],
  limit: number,
): string[] {
  if (limit <= 0) return [];
  const inList = new Set(available);
  const ranked = Object.keys(freq)
    .filter((repo) => inList.has(repo))
    .sort((a, b) => {
      const byCount = freq[b].count - freq[a].count;
      return byCount !== 0 ? byCount : freq[b].lastUsed - freq[a].lastUsed;
    });
  const chosen: string[] = [];
  const seen = new Set<string>();
  const take = (repo: string) => {
    if (chosen.length < limit && !seen.has(repo)) {
      chosen.push(repo);
      seen.add(repo);
    }
  };
  ranked.forEach(take);
  available.forEach(take);
  return chosen;
}

export async function loadRepoFrequency(
  ownerId: string,
): Promise<RepoFrequency> {
  if (!ownerId) return {};
  const raw = await AsyncStorage.getItem(storageKey(ownerId)).catch(() => null);
  if (!raw) return {};
  try {
    const parsed: unknown = JSON.parse(raw);
    return isRepoFrequency(parsed) ? parsed : {};
  } catch {
    return {};
  }
}

// Record one use of `repo` in `ownerId`'s workspace and return the updated map
// (best-effort: a storage failure still returns the in-memory bump).
export async function recordRepoUse(
  ownerId: string,
  repo: string,
  now: number,
): Promise<RepoFrequency> {
  const next = bumpRepo(await loadRepoFrequency(ownerId), repo, now);
  if (ownerId) {
    await AsyncStorage.setItem(storageKey(ownerId), JSON.stringify(next)).catch(
      () => undefined,
    );
  }
  return next;
}

function isRepoFrequency(value: unknown): value is RepoFrequency {
  if (typeof value !== "object" || value === null) return false;
  return Object.values(value as Record<string, unknown>).every(
    (entry) =>
      typeof entry === "object" &&
      entry !== null &&
      typeof (entry as RepoUse).count === "number" &&
      typeof (entry as RepoUse).lastUsed === "number",
  );
}
