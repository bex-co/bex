import { createContext, useContext } from "react";
import type { BuildSourceView } from "@/features/build-sources/types";

export interface BuildSourceDetailContextValue {
  buildSource: BuildSourceView;
  refetch: () => Promise<BuildSourceView | null>;
  openTriggerBuild: () => void;
}

export const BuildSourceDetailContext =
  createContext<BuildSourceDetailContextValue | null>(null);

export function useBuildSourceDetailContext(): BuildSourceDetailContextValue {
  const ctx = useContext(BuildSourceDetailContext);
  if (!ctx) {
    throw new Error(
      "useBuildSourceDetailContext must be used inside BuildSourceLayout",
    );
  }
  return ctx;
}
