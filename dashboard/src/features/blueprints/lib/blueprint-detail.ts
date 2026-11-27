import { createContext, useContext } from "react";
import type { BlueprintView } from "@/features/blueprints/types";

/**
 * What a Blueprint's layout route hands each of its pages (w3/m677): the
 * Blueprint it read, addressed by the API's exs- id whatever spelling the URL
 * carries (w2/m269). The layout mounts its `<Outlet/>` only once the Blueprint
 * is loaded and its workspace is the selection, so a page never renders
 * without one.
 */
export interface BlueprintDetail {
  blueprint: BlueprintView;
  /** The Blueprint's id as the URL spells it (a `blp-` bookmark keeps its
   *  spelling), for links between its pages. */
  linkId: string;
}

const BlueprintDetailContext = createContext<BlueprintDetail | null>(null);

export const BlueprintDetailProvider = BlueprintDetailContext.Provider;

/** The open Blueprint, for a page under `/blueprints/$blueprintId`. */
export function useBlueprintDetail(): BlueprintDetail {
  const detail = useOptionalBlueprintDetail();
  if (!detail) {
    throw new Error("useBlueprintDetail outside the Blueprint layout");
  }
  return detail;
}

/**
 * The open Blueprint once its layout has it, else null: a page's pending
 * frame, which also paints while the layout is still reading it, sizes
 * itself from it when it can (w3/m680).
 */
export function useOptionalBlueprintDetail(): BlueprintDetail | null {
  return useContext(BlueprintDetailContext);
}
