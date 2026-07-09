import { useContext } from "react";
import { WorkspaceContext } from "./context";

/** The selected workspace + switcher data — see WorkspaceProvider. */
export const useWorkspace = () => useContext(WorkspaceContext);
