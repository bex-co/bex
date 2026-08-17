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
import { authManager, useAuth } from "@/features/auth/auth-provider";
import {
  InviteFlowController,
  type InviteFlowState,
} from "./invite-controller";
import {
  ApolloInviteAcceptanceClient,
  refreshInviteWorkspaces,
} from "./graphql-client";
import { createExpoInviteStore } from "./expo-invite-storage";

type InviteContextValue = {
  state: InviteFlowState;
  capture: (value: unknown) => Promise<boolean>;
  accept: () => Promise<void>;
};

const InviteContext = createContext<InviteContextValue | null>(null);

export function InviteProvider({ children }: { children: ReactNode }) {
  const apollo = useApolloClient();
  const { state: auth } = useAuth();
  const [state, setState] = useState<InviteFlowState>({ status: "loading" });
  const controllerRef = useRef<InviteFlowController | null>(null);
  if (!controllerRef.current) {
    controllerRef.current = new InviteFlowController(
      createExpoInviteStore(),
      new ApolloInviteAcceptanceClient(apollo),
      () => refreshInviteWorkspaces(apollo),
      setState,
    );
  }
  const controller = controllerRef.current;
  const subject = auth.status === "signedIn" ? auth.session.subject : null;

  useEffect(() => {
    void controller.bootstrap(subject);
  }, [controller, subject]);

  useEffect(
    () =>
      authManager.registerSessionClearHook(() =>
        controller.clear().catch(() => undefined),
      ),
    [controller],
  );

  const capture = useCallback(
    (value: unknown) => controller.capture(value, subject),
    [controller, subject],
  );
  const accept = useCallback(async () => {
    if (subject) await controller.accept(subject);
  }, [controller, subject]);
  const value = useMemo(
    () => ({ state, capture, accept }),
    [accept, capture, state],
  );
  return (
    <InviteContext.Provider value={value}>{children}</InviteContext.Provider>
  );
}

export function useInvite(): InviteContextValue {
  const value = useContext(InviteContext);
  if (!value) throw new Error("useInvite must be used inside InviteProvider");
  return value;
}
