import { useNavigate } from "@tanstack/react-router";
import { ChevronDown } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { useTranslations } from "@/common/hooks/use-translations";
import { useTriggerDeploy } from "@/features/services/hooks/use-trigger-deploy";
import { serviceBaseForType } from "@/features/services/lib/service-base";
import type { ServiceView } from "@/features/services/types";

export interface ManualDeployButtonProps {
  service: ServiceView;
  /** Whether any action is already in flight for this service. */
  pending: boolean;
}

/**
 * Render's "Manual Deploy" header dropdown ("Deploy latest commit" /
 * "Deploy latest image", a divider, then "Restart service").
 *
 * Both "Deploy" and "Restart service" route through the same `triggerDeploy`
 * mutation (w2/m30 consolidation) so every rollout — including a restart —
 * opens a deploy-history row in the Events tab.
 *
 * For repo-backed services "Restart service" triggers a rebuild from Branch
 * HEAD (bex has no way to restart pods without a new build — any spec change
 * increments the generation and unconditionally re-enters the build path).
 * For image-backed services it re-pulls and restarts the containers in place.
 *
 * "Clear build cache & deploy" mirrors Render's dropdown item (w3/m46). bex's
 * builds are always cache-free (ephemeral BuildKit Jobs, no --cache-from/-to),
 * so it sends Render's clearCache="clear" — an enum-validated no-op the backend
 * accepts across REST/GraphQL/MCP — and rebuilds from a clean slate exactly like
 * "Deploy latest commit". It is kept for surface parity with Render (and the CLI,
 * which always sends the flag). "Deploy a specific commit" (per-commit targeting
 * via commitId) stays an API-only feature for now.
 */
export function ManualDeployButton({
  service,
  pending,
}: ManualDeployButtonProps) {
  const { t } = useTranslations();
  const { deploying, trigger } = useTriggerDeploy();
  const navigate = useNavigate();
  // A static_site's deploys live under /static (Render parity, w5/m57).
  const base = serviceBaseForType(service.type);
  const busy = deploying || pending;
  const repoBacked = !!service.repo;

  const deployLabel = repoBacked
    ? t("services.deployMenuLatestCommit")
    : t("services.deployMenuLatestImage");

  async function handleDeploy(opts?: { clearCache?: boolean }) {
    // "Deploy", "Clear build cache & deploy", and "Restart" all go through
    // triggerDeploy — the same mutation — so each opens a deploy-history row
    // (w2/m30). clearCache is a no-op on bex (cache-free builds) but is sent for
    // Render/CLI parity. Restart passes no extra options: for image-backed
    // services this re-pulls and restarts; for repo-backed it rebuilds from HEAD.
    const deployId = opts?.clearCache
      ? await trigger(service.id, { clearCache: "clear" })
      : await trigger(service.id);
    // Render lands the user straight on the new deploy's page (w9/m1/t004); a
    // failed trigger already toasted and has no deploy id to navigate to.
    if (deployId) {
      void navigate({
        to: `${base}/$serviceId/deploys/$deployId`,
        params: { serviceId: service.id, deployId },
      });
    }
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button size="sm" disabled={busy}>
          {t("services.eventsManualDeploy")}
          <ChevronDown className="size-3.5" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem disabled={busy} onSelect={() => void handleDeploy()}>
          {deployLabel}
        </DropdownMenuItem>
        <DropdownMenuItem
          disabled={busy}
          onSelect={() => void handleDeploy({ clearCache: true })}
        >
          {t("services.deployMenuClearCache")}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem disabled={busy} onSelect={() => void handleDeploy()}>
          {t("services.deployMenuRestart")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
