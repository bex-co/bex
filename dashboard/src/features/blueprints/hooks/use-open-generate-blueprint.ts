import { useCallback } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useTranslations } from "@/common/hooks/use-translations";
import type { BlueprintExport } from "@/features/blueprints/lib/blueprint-export";

/**
 * Generate Blueprint's two halves for an entry point (w3/m681): why the role
 * cannot use it, or undefined, and the navigation that opens
 * `/blueprint/generate` with the resources in its state, as Render's entry
 * points do. Render disables every entry without `createBlueprint`; bex
 * gates creating a Blueprint on can_create, so a Viewer reads that reason.
 */
export function useOpenGenerateBlueprint(): {
  reason: string | undefined;
  open: (handed: BlueprintExport) => void;
} {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const reasonKey = useCapabilities().reasonKey("can_create");
  const open = useCallback(
    (handed: BlueprintExport) => {
      void navigate({
        to: "/blueprint/generate",
        state: { blueprintExport: handed },
      });
    },
    [navigate],
  );
  return { reason: reasonKey ? t(reasonKey) : undefined, open };
}
