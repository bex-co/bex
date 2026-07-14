import { useEffect, useState, type Dispatch, type SetStateAction } from "react";

export interface UseFetchedListResult<T> {
  data: T[];
  loading: boolean;
  error: boolean;
  /** Lets a caller apply an optimistic update (e.g. filter out a just-revoked row). */
  setData: Dispatch<SetStateAction<T[]>>;
  /** Re-runs the fetch from scratch — for a manual reload, not needed after an
   * optimistic `setData` already reflects a mutation's outcome. */
  refetch: () => void;
}

/**
 * `GET url` on mount (and again each `refetch()`), tracking loading/error —
 * the fetch-a-JSON-array shape shared by every Settings card that reads a
 * dashboard SSR server-fn route directly (not GraphQL): `useConnectedAgents`,
 * `useActiveSessions`.
 */
export function useFetchedList<T>(url: string): UseFetchedListResult<T> {
  const [data, setData] = useState<T[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [generation, setGeneration] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(false);
    fetch(url)
      .then((res) => {
        if (!res.ok) throw new Error(`failed to fetch ${url}`);
        return res.json() as Promise<T[]>;
      })
      .then((json) => {
        if (!cancelled) setData(json);
      })
      .catch(() => {
        if (!cancelled) setError(true);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
    // Deliberately excludes `url` changing mid-lifetime — every caller passes
    // a literal route path, never a value that varies across renders.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [generation]);

  return {
    data,
    loading,
    error,
    setData,
    refetch: () => setGeneration((g) => g + 1),
  };
}
