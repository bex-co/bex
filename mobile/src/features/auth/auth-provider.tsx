import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import * as WebBrowser from "expo-web-browser";
import { randomUUID } from "expo-crypto";
import { ExpoOAuthTransport } from "./expo-oauth-transport";
import { SecureAuthStorage } from "./secure-storage";
import { SessionManager } from "./session-manager";
import { mobileConfig } from "./config";
import type { AuthState } from "./types";
import { resetIdentityBoundary } from "@/common/apollo/data-boundary";

WebBrowser.maybeCompleteAuthSession();

export const authManager = new SessionManager(
  new SecureAuthStorage(),
  new ExpoOAuthTransport(),
  mobileConfig,
  Date.now,
  randomUUID,
  resetIdentityBoundary,
);

type AuthContextValue = {
  state: AuthState;
  signIn: () => Promise<void>;
  completeSignIn: (redirectUrl: string) => Promise<void>;
  signOut: () => Promise<void>;
  retryRestore: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>(authManager.getState());
  useEffect(() => {
    const unsubscribe = authManager.subscribe(setState);
    authManager.restore().catch(() => undefined);
    return unsubscribe;
  }, []);
  const signIn = useCallback(() => authManager.signIn(), []);
  const completeSignIn = useCallback(
    (redirectUrl: string) => authManager.completeSignIn(redirectUrl),
    [],
  );
  const signOut = useCallback(() => authManager.signOut(), []);
  const retryRestore = useCallback(() => authManager.restore(), []);
  const value = useMemo(
    () => ({ state, signIn, completeSignIn, signOut, retryRestore }),
    [completeSignIn, retryRestore, signIn, signOut, state],
  );
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const value = useContext(AuthContext);
  if (!value) throw new Error("useAuth must be used inside AuthProvider");
  return value;
}
