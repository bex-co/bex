import { useMemo } from "react";
import { useRouterState } from "@tanstack/react-router";
import { useIsHydrated } from "@/common/hooks/use-is-hydrated";
import {
  readBlueprintExport,
  type BlueprintExport,
} from "@/features/blueprints/lib/blueprint-export";

/**
 * The export Generate Blueprint handed `/blueprint/generate` (w3/m681). The
 * navigation's state lives in the browser's history, which the server render
 * cannot see, so until the page has hydrated it is `undefined` (not yet
 * known); after, the export or null when none was handed over.
 */
export function useHandedBlueprintExport(): BlueprintExport | null | undefined {
  const hydrated = useIsHydrated();
  // The history entry's own object, so the selection keeps its identity
  // across unrelated router updates.
  const raw = useRouterState({
    select: (s) => s.location.state.blueprintExport,
  });
  const handed = useMemo(
    () => readBlueprintExport({ blueprintExport: raw }),
    [raw],
  );
  return hydrated ? handed : undefined;
}
