import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { CurrentUserProvider } from "@/features/profile/current-user-provider";
import { AppDrawer } from "./app-drawer";

type AppDrawerContextValue = {
  openDrawer: () => void;
};

const AppDrawerContext = createContext<AppDrawerContextValue | null>(null);

/**
 * Hosts a single {@link AppDrawer} for the whole authenticated tab group. Every
 * screen opens the same instance through {@link useAppDrawer}, so switching
 * tabs, following a deep link, or opening a nested detail never remounts or
 * duplicates the drawer. Mounted once inside `WorkspaceProvider` so its
 * workspace/personal content shares the reviewed tenant boundary.
 */
export function AppDrawerProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const openDrawer = useCallback(() => setOpen(true), []);
  const closeDrawer = useCallback(() => setOpen(false), []);
  const value = useMemo(() => ({ openDrawer }), [openDrawer]);
  return (
    <AppDrawerContext.Provider value={value}>
      <CurrentUserProvider>
        <AppDrawer open={open} onOpen={openDrawer} onClose={closeDrawer}>
          {children}
        </AppDrawer>
      </CurrentUserProvider>
    </AppDrawerContext.Provider>
  );
}

export function useAppDrawer(): AppDrawerContextValue {
  const value = useContext(AppDrawerContext);
  if (!value) {
    throw new Error("useAppDrawer must be used inside an AppDrawerProvider");
  }
  return value;
}
