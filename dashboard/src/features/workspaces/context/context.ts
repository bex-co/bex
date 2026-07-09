import { createContext } from "react";
import type { WorkspaceView } from "@/features/workspaces/types";

export interface WorkspaceContextValue {
  /** The caller's workspaces (switcher's list). */
  workspaces: WorkspaceView[];
  /** The selected workspace's id, or null before the first workspace loads. */
  currentWorkspaceId: string | null;
  currentWorkspace: WorkspaceView | null;
  /** Switch the selected workspace (persisted across sessions). */
  setCurrentWorkspaceId: (id: string) => void;
  loading: boolean;
  error: Error | undefined;
  /** Re-run the workspaces list (callers refresh after create/rename/delete). */
  refetch: () => Promise<unknown>;
}

const noop = () => undefined;

const initialState: WorkspaceContextValue = {
  workspaces: [],
  currentWorkspaceId: null,
  currentWorkspace: null,
  setCurrentWorkspaceId: noop,
  loading: true,
  error: undefined,
  refetch: () => Promise.resolve(),
};

export const WorkspaceContext = createContext<WorkspaceContextValue>(initialState);
