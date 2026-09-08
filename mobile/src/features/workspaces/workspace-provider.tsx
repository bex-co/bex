import AsyncStorage from "@react-native-async-storage/async-storage";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useApolloClient } from "@apollo/client/react";
import { MobileWorkspacesDocument } from "@/generated-graphql";
import {
  dataBoundary,
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
  status: "loading" | "ready" | "choose" | "empty" | "error";
  workspaces: MobileWorkspace[];
  selected: MobileWorkspace | null;
  switching: boolean;
  offline: boolean;
  switchWorkspace: (workspaceId: string) => Promise<void>;
  refreshMembership: () => Promise<MobileWorkspace | null>;
  retry: () => Promise<void>;
};

const WorkspaceContext = createContext<WorkspaceContextValue | null>(null);

export function WorkspaceProvider({ children }: { children: ReactNode }) {
  const { state: auth } = useAuth();
  const network = useNetworkState();
  const client = useApolloClient();
  const sessionId = auth.status === "signedIn" ? auth.session.sessionId : null;
  const [workspaces, setWorkspaces] = useState<MobileWorkspace[]>([]);
  const [selected, setSelected] = useState<MobileWorkspace | null>(null);
  const [status, setStatus] =
    useState<WorkspaceContextValue["status"]>("loading");
  const [switching, setSwitching] = useState(false);
  const selectedRef = useRef<MobileWorkspace | null>(null);
  const revision = useRef(0);
  const selectionRequired = useRef(false);
  const flight = useRef<Promise<MobileWorkspace | null> | null>(null);

  const refreshMembership = useCallback((): Promise<MobileWorkspace | null> => {
    if (flight.current) return flight.current;
    if (!sessionId) return Promise.resolve(null);
    const version = revision.current;
    const current = () => version === revision.current;
    const run = async () => {
      try {
        const result = await client.query({
          query: MobileWorkspacesDocument,
          fetchPolicy: "no-cache",
        });
        if (!current()) return null;
        if (!result.data) throw new Error("workspace check unavailable");
        const available = normalizeWorkspaces(result.data.workspaces);
        const key = workspaceSelectionKey(sessionId);
        const persisted =
          selectedRef.current?.id ??
          (await AsyncStorage.getItem(key).catch(() => null));
        if (!current()) return null;
        const removed =
          selectedRef.current !== null &&
          !available.some(
            (workspace) => workspace.id === selectedRef.current?.id,
          );
        if (removed) selectionRequired.current = true;
        const next = selectionRequired.current
          ? null
          : chooseWorkspace(available, persisted);
        if (removed) {
          selectedRef.current = null;
          setSelected(null);
          setSwitching(true);
          await dataBoundary.reset(next?.id ?? null);
          if (!current()) return null;
        } else if (next) {
          initializeWorkspaceBoundary(next.id);
        }
        selectedRef.current = next;
        setWorkspaces(available);
        setSelected(next);
        setStatus(next ? "ready" : available.length ? "choose" : "empty");
        setSwitching(false);
        if (next && next.id !== persisted)
          await AsyncStorage.setItem(key, next.id).catch(() => undefined);
        else if (!next && persisted)
          await AsyncStorage.removeItem(key).catch(() => undefined);
        return next;
      } catch (error) {
        if (current()) {
          setStatus("error");
          setSwitching(false);
        }
        throw error;
      }
    };
    const pending = run();
    flight.current = pending;
    void pending.then(
      () => {
        if (flight.current === pending) flight.current = null;
      },
      () => {
        if (flight.current === pending) flight.current = null;
      },
    );
    return pending;
  }, [client, sessionId]);

  useEffect(() => {
    void refreshMembership().catch(() => undefined);
    return () => {
      revision.current += 1;
      flight.current = null;
    };
  }, [refreshMembership]);

  const switchWorkspace = useCallback(
    async (workspaceId: string) => {
      if (!sessionId || selectedRef.current?.id === workspaceId) return;
      const next = workspaces.find((workspace) => workspace.id === workspaceId);
      if (!next) return;
      const version = ++revision.current;
      flight.current = null;
      setSwitching(true);
      try {
        await resetWorkspaceBoundary(next.id);
        if (version !== revision.current) return;
        selectedRef.current = next;
        selectionRequired.current = false;
        setSelected(next);
        setStatus("ready");
        await AsyncStorage.setItem(
          workspaceSelectionKey(sessionId),
          next.id,
        ).catch(() => undefined);
      } finally {
        if (version === revision.current) setSwitching(false);
      }
    },
    [sessionId, workspaces],
  );

  const retry = useCallback(async () => {
    await refreshMembership();
  }, [refreshMembership]);
  const value = useMemo(
    () => ({
      status,
      workspaces,
      selected,
      switching,
      offline: network === "offline",
      switchWorkspace,
      refreshMembership,
      retry,
    }),
    [
      network,
      retry,
      selected,
      status,
      switchWorkspace,
      refreshMembership,
      switching,
      workspaces,
    ],
  );
  return (
    <WorkspaceContext.Provider value={value}>
      {children}
    </WorkspaceContext.Provider>
  );
}

export function useWorkspace(): WorkspaceContextValue {
  const value = useContext(WorkspaceContext);
  if (!value)
    throw new Error("useWorkspace must be used inside WorkspaceProvider");
  return value;
}
