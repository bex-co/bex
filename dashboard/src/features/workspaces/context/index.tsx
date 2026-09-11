import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { WorkspaceContext } from "./context";
import { useWorkspaces } from "@/features/workspaces/hooks/use-workspaces";
import {
  clearPersistedWorkspaceId,
  persistWorkspaceId,
} from "@/features/workspaces/lib/selection";
import { bumpAccessGeneration } from "@/features/capabilities/lib/access-generation";

/**
 * Scopes every dashboard page (services, databases, env vars, metrics) to one
 * workspace (w6/m3): reads the caller's workspace list once, restores the
 * server-selected cookie, and falls back to the first workspace when there is
 * no stored selection or the stored one no longer exists (e.g. it was just
 * deleted). A successful empty membership clears the selection and routes to
 * `/new/workspace` without treating transport errors as removal (w6/m144).
 * Mounted once in RootProvider so every authenticated page shares one selection.
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
  const { workspaces, loading, error, ready, refetch } = useWorkspaces();
  const navigate = useNavigate();
  // User/cookie preference. Effective selection is derived below so membership
  // changes never need setState-in-effect (w6/m144).
  const [preferredId, setPreferredId] = useState<string | null>(
    initialWorkspaceId,
  );
  const handledEmptyRef = useRef(false);
  const handledFallbackRef = useRef<string | null>(null);

  const selectedId = useMemo(() => {
    if (!ready) return preferredId;
    if (workspaces.length === 0) return null;
    if (preferredId && workspaces.some((w) => w.id === preferredId)) {
      return preferredId;
    }
    return workspaces[0].id;
  }, [ready, workspaces, preferredId]);

  // Persist + navigate as external side effects of a confirmed membership
  // transition. Does not call setState — selectedId is already derived.
  useEffect(() => {
    if (!ready) return;

    if (workspaces.length === 0) {
      handledFallbackRef.current = null;
      if (!handledEmptyRef.current) {
        handledEmptyRef.current = true;
        clearPersistedWorkspaceId();
        bumpAccessGeneration();
        onWorkspaceChange?.();
        void navigate({
          to: "/new/workspace",
          search: { attempt: undefined },
          replace: true,
        });
      }
      return;
    }

    handledEmptyRef.current = false;
    const preferredExists =
      preferredId !== null && workspaces.some((w) => w.id === preferredId);
    if (!preferredExists) {
      const fallback = workspaces[0].id;
      if (handledFallbackRef.current !== fallback) {
        handledFallbackRef.current = fallback;
        persistWorkspaceId(fallback);
        bumpAccessGeneration();
        onWorkspaceChange?.();
      }
    } else {
      handledFallbackRef.current = null;
    }
  }, [ready, workspaces, preferredId, navigate, onWorkspaceChange]);

  const setCurrentWorkspaceId = useCallback(
    (id: string) => {
      setPreferredId(id);
      persistWorkspaceId(id);
      bumpAccessGeneration();
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
      currentWorkspace,
      currentWorkspaceId: selectedId,
      setCurrentWorkspaceId,
      loading,
      error,
      refetch,
    }),
    [
      workspaces,
      currentWorkspace,
      selectedId,
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
