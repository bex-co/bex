import { useCallback, useEffect, useMemo, useState } from "react";
import { WorkspaceContext } from "./context";
import { useWorkspaces } from "@/features/workspaces/hooks/use-workspaces";

// Persists the switcher's selection across sessions/tabs (Render's dashboard
// remembers the last workspace too). Read only after mount (see the effect
// below) so the server-rendered pass and the client's first pass agree — no
// hydration mismatch, same pattern as ThemeProvider (common/providers/theme-provider).
const STORAGE_KEY = "bex.selectedWorkspaceId";

function readStoredId(): string | null {
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem(STORAGE_KEY);
}

/**
 * Scopes every dashboard page (services, databases, env vars, metrics) to one
 * workspace (w6/m3): reads the caller's workspace list once, restores the
 * last selection from localStorage, and falls back to the first workspace
 * when there is no stored selection or the stored one no longer exists (e.g.
 * it was just deleted). Mounted once in DashboardLayout so every authenticated
 * page shares one selection.
 */
export function WorkspaceProvider({ children }: { children: React.ReactNode }) {
  const { workspaces, loading, error, refetch } = useWorkspaces();
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [hydrated, setHydrated] = useState(false);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- Intentional for hydration tracking (matches ThemeProvider)
    setHydrated(true);
    setSelectedId(readStoredId());
  }, []);

  useEffect(() => {
    if (!hydrated || loading || workspaces.length === 0) return;
    const stillExists = workspaces.some((w) => w.id === selectedId);
    // eslint-disable-next-line react-hooks/set-state-in-effect -- Intentional: falls back to the first workspace once the list resolves
    if (!stillExists) setSelectedId(workspaces[0].id);
  }, [hydrated, loading, workspaces, selectedId]);

  const setCurrentWorkspaceId = useCallback((id: string) => {
    setSelectedId(id);
    if (typeof window !== "undefined") {
      window.localStorage.setItem(STORAGE_KEY, id);
    }
  }, []);

  const currentWorkspace = useMemo(
    () => workspaces.find((w) => w.id === selectedId) ?? null,
    [workspaces, selectedId],
  );

  const value = useMemo(
    () => ({
      workspaces,
      currentWorkspaceId: selectedId,
      currentWorkspace,
      setCurrentWorkspaceId,
      loading,
      error,
      refetch,
    }),
    [
      workspaces,
      selectedId,
      currentWorkspace,
      setCurrentWorkspaceId,
      loading,
      error,
      refetch,
    ],
  );

  return (
    <WorkspaceContext.Provider value={value}>{children}</WorkspaceContext.Provider>
  );
}

/* eslint-disable-next-line react-refresh/only-export-components */
export { useWorkspace } from "./hooks";
