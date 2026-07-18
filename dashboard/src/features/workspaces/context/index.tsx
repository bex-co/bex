import { useCallback, useEffect, useMemo, useState } from "react";
import { WorkspaceContext } from "./context";
import { useWorkspaces } from "@/features/workspaces/hooks/use-workspaces";
import {
  WORKSPACE_SELECTION_KEY,
  persistWorkspaceId,
} from "@/features/workspaces/lib/selection";

// Legacy localStorage selection used only when the SSR-readable workspace
// cookie is absent. Reading it after mount preserves the server/client first
// render; the provider then promotes it to the cookie for future route loaders.
function readStoredId(): string | null {
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem(WORKSPACE_SELECTION_KEY);
}

/**
 * Scopes every dashboard page (services, databases, env vars, metrics) to one
 * workspace (w6/m3): reads the caller's workspace list once, restores the
 * server-selected cookie (or legacy localStorage value), and falls back to the
 * first workspace when there is no stored selection or the stored one no longer
 * exists (e.g. it was just deleted). Mounted once in DashboardLayout so every
 * authenticated page shares one selection.
 */
export function WorkspaceProvider({
  children,
  initialWorkspaceId = null,
  onWorkspaceChange,
}: {
  children: React.ReactNode;
  initialWorkspaceId?: string | null;
  onWorkspaceChange?: () => void;
}) {
  const { workspaces, loading, error, refetch } = useWorkspaces();
  const [selectedId, setSelectedId] = useState<string | null>(
    initialWorkspaceId,
  );
  const [hydrated, setHydrated] = useState(false);

  useEffect(() => {
    const storedId = readStoredId();
    const restoredId = initialWorkspaceId ?? storedId;
    // eslint-disable-next-line react-hooks/set-state-in-effect -- Intentional for hydration tracking (matches ThemeProvider)
    setHydrated(true);
    setSelectedId(restoredId);
    // Existing users can have only the legacy localStorage selection. Promote
    // it to the SSR-readable cookie, then refresh route loaders/heads once.
    if (!initialWorkspaceId && storedId) {
      persistWorkspaceId(storedId);
      onWorkspaceChange?.();
    }
  }, [initialWorkspaceId, onWorkspaceChange]);

  useEffect(() => {
    if (!hydrated || loading || workspaces.length === 0) return;
    const stillExists = workspaces.some((w) => w.id === selectedId);
    if (!stillExists) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- Intentional: falls back to the first workspace once the list resolves
      setSelectedId(workspaces[0].id);
      persistWorkspaceId(workspaces[0].id);
      onWorkspaceChange?.();
    }
  }, [hydrated, loading, onWorkspaceChange, workspaces, selectedId]);

  const setCurrentWorkspaceId = useCallback(
    (id: string) => {
      setSelectedId(id);
      persistWorkspaceId(id);
      onWorkspaceChange?.();
    },
    [onWorkspaceChange],
  );

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
    <WorkspaceContext.Provider value={value}>
      {children}
    </WorkspaceContext.Provider>
  );
}

/* eslint-disable-next-line react-refresh/only-export-components */
export { useWorkspace } from "./hooks";
