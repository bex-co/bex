import AsyncStorage from "@react-native-async-storage/async-storage";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { useQuery } from "@apollo/client/react";
import { MobileWorkspacesDocument } from "@/generated-graphql";
import {
  initializeWorkspaceBoundary,
  resetWorkspaceBoundary,
} from "@/common/apollo/data-boundary";
import { useAuth } from "@/features/auth/auth-provider";
import { useNetworkState } from "@/common/apollo/network-state";
import {
  chooseWorkspace,
  normalizeWorkspaces,
  workspaceSelectionKey,
  type MobileWorkspace,
} from "./workspace-selection";

type WorkspaceContextValue = {
  status: "loading" | "ready" | "empty" | "error";
  workspaces: MobileWorkspace[];
  selected: MobileWorkspace | null;
  switching: boolean;
  offline: boolean;
  switchWorkspace: (workspaceId: string) => Promise<void>;
  retry: () => Promise<void>;
};

const WorkspaceContext = createContext<WorkspaceContextValue | null>(null);

export function WorkspaceProvider({ children }: { children: ReactNode }) {
  const { state: auth } = useAuth();
  const network = useNetworkState();
  const [workspaces, setWorkspaces] = useState<MobileWorkspace[]>([]);
  const [selected, setSelected] = useState<MobileWorkspace | null>(null);
  const [switching, setSwitching] = useState(false);
  const { data, loading, error, refetch } = useQuery(MobileWorkspacesDocument, {
    fetchPolicy: "network-only",
    notifyOnNetworkStatusChange: true,
  });
  const sessionId = auth.status === "signedIn" ? auth.session.sessionId : null;

  useEffect(() => {
    if (!sessionId || !data) return;
    let cancelled = false;
    const available = normalizeWorkspaces(data.workspaces);
    setWorkspaces(available);
    void (async () => {
      const key = workspaceSelectionKey(sessionId);
      const persisted = await AsyncStorage.getItem(key).catch(() => null);
      const next = chooseWorkspace(available, persisted);
      if (cancelled || !next) {
        if (!cancelled) setSelected(null);
        return;
      }
      initializeWorkspaceBoundary(next.id);
      setSelected(next);
      await AsyncStorage.setItem(key, next.id).catch(() => undefined);
    })();
    return () => {
      cancelled = true;
    };
  }, [data, sessionId]);

  const switchWorkspace = useCallback(
    async (workspaceId: string) => {
      if (!sessionId || selected?.id === workspaceId) return;
      const next = workspaces.find((workspace) => workspace.id === workspaceId);
      if (!next) return;
      setSwitching(true);
      try {
        await resetWorkspaceBoundary(next.id);
        setSelected(next);
        await AsyncStorage.setItem(
          workspaceSelectionKey(sessionId),
          next.id,
        ).catch(() => undefined);
      } finally {
        setSwitching(false);
      }
    },
    [selected?.id, sessionId, workspaces],
  );

  const retry = useCallback(async () => {
    await refetch();
  }, [refetch]);

  const status: WorkspaceContextValue["status"] = error
    ? "error"
    : loading || (workspaces.length > 0 && !selected)
      ? "loading"
      : workspaces.length === 0
        ? "empty"
        : "ready";
  const value = useMemo(
    () => ({
      status,
      workspaces,
      selected,
      switching,
      offline: network === "offline",
      switchWorkspace,
      retry,
    }),
    [network, retry, selected, status, switchWorkspace, switching, workspaces],
  );
  return (
    <WorkspaceContext.Provider value={value}>
      {children}
    </WorkspaceContext.Provider>
  );
}

export function useWorkspace(): WorkspaceContextValue {
  const value = useContext(WorkspaceContext);
  if (!value) {
    throw new Error("useWorkspace must be used inside WorkspaceProvider");
  }
  return value;
}
