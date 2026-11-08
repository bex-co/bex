import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { Pause, Pencil, Play, Trash2, UserCheck, Zap } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import type { AutomationView } from "@/features/agent-automations/hooks/use-agent-automations";
import {
  pauseToggleKind,
  type AutomationActions,
} from "@/features/agent-automations/lib/display";

interface AutomationActionBarProps {
  /** Null while the automation loads: every action shows, none is enabled. */
  automation: AutomationView | null;
  actions: AutomationActions;
  busy: boolean;
  onRunNow: () => void;
  onToggle: () => void;
  onTakeOver: () => Promise<boolean>;
  onDelete: () => Promise<boolean>;
}

/**
 * Run now, Pause or Resume, Edit, Take over, Delete (ADR101 D4, D9). All five
 * render for everyone and are disabled by role (the creator edits; the creator
 * or an admin runs, pauses and deletes; only an admin takes over), so the bar
 * keeps one geometry before and after the viewer's role is known.
 */
export function AutomationActionBar({
  automation,
  actions,
  busy,
  onRunNow,
  onToggle,
  onTakeOver,
  onDelete,
}: AutomationActionBarProps) {
  const { t } = useTranslations();
  const [confirm, setConfirm] = useState<"takeOver" | "delete" | null>(null);
  const loaded = automation != null;
  const kind = automation ? pauseToggleKind(automation) : "pause";
  const platformPaused = Boolean(automation?.pausedReason);
  const name = automation?.name ?? "";

  async function confirmAndClose(work: () => Promise<boolean>) {
    const ok = await work();
    if (ok) setConfirm(null);
  }

  return (
    <div
      className="flex flex-wrap gap-2"
      role="group"
      aria-label={t("agentAutomations.actionsLabel")}
      data-skeleton-region="actions"
    >
      <Button
        variant="outline"
        disabled={!loaded || !actions.operate || platformPaused || busy}
        title={
          platformPaused ? t("agentAutomations.runNowPausedHint") : undefined
        }
        onClick={onRunNow}
      >
        <Zap />
        {t("agentAutomations.runNow")}
      </Button>
      <Button
        variant="outline"
        disabled={!loaded || !actions.operate || busy}
        onClick={onToggle}
      >
        {kind === "pause" ? <Pause /> : <Play />}
        {t(
          kind === "pause"
            ? "agentAutomations.pause"
            : "agentAutomations.resume",
        )}
      </Button>
      {loaded && actions.edit ? (
        <Button variant="outline" asChild>
          <Link
            to="/automations/$automationId/edit"
            params={{ automationId: automation.id }}
          >
            <Pencil />
            {t("agentAutomations.edit")}
          </Link>
        </Button>
      ) : (
        <Button
          variant="outline"
          disabled
          title={loaded ? t("agentAutomations.editCreatorOnly") : undefined}
        >
          <Pencil />
          {t("agentAutomations.edit")}
        </Button>
      )}
      <Button
        variant="outline"
        disabled={!loaded || !actions.takeOver || busy}
        title={
          loaded && !actions.takeOver
            ? t("agentAutomations.takeOverAdminOnly")
            : undefined
        }
        onClick={() => setConfirm("takeOver")}
      >
        <UserCheck />
        {t("agentAutomations.takeOver")}
      </Button>
      <Button
        variant="outline"
        className="text-destructive hover:text-destructive"
        disabled={!loaded || !actions.remove || busy}
        onClick={() => setConfirm("delete")}
      >
        <Trash2 />
        {t("agentAutomations.delete")}
      </Button>

      <ConfirmDialog
        open={confirm === "takeOver"}
        onOpenChange={(open) => setConfirm(open ? "takeOver" : null)}
        title={t("agentAutomations.takeOverTitle", { name })}
        description={t("agentAutomations.takeOverBody")}
        confirmLabel={t("agentAutomations.takeOver")}
        destructive={false}
        closeOnConfirm={false}
        pending={busy}
        onConfirm={() => void confirmAndClose(onTakeOver)}
      />
      <ConfirmDialog
        open={confirm === "delete"}
        onOpenChange={(open) => setConfirm(open ? "delete" : null)}
        title={t("agentAutomations.deleteTitle", { name })}
        description={t("agentAutomations.deleteBody")}
        confirmLabel={t("agentAutomations.delete")}
        closeOnConfirm={false}
        pending={busy}
        onConfirm={() => void confirmAndClose(onDelete)}
      />
    </div>
  );
}
