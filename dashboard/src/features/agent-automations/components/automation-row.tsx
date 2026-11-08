import type { ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import { Switch } from "@/common/components/ui/switch";
import { useTranslations } from "@/common/hooks/use-translations";
import { InstantLabel } from "@/features/agent-automations/components/instant-label";
import { AutomationStatusBadge } from "@/features/agent-automations/components/automation-status";
import type { AutomationView } from "@/features/agent-automations/hooks/use-agent-automations";
import { automationStatus } from "@/features/agent-automations/lib/display";
import { describeSchedule } from "@/features/agent-automations/lib/schedule";

/**
 * One list row's geometry. The ready row and the skeleton row both fill its
 * slots, so a row is the same height pending and ready: a 24px name line, a
 * 20px trigger line and a 16px timing line (each truncated to one line), and
 * the switch.
 */
export function AutomationRowShell({
  name,
  trigger,
  timing,
  control,
}: {
  name: ReactNode;
  trigger: ReactNode;
  timing: ReactNode;
  control: ReactNode;
}) {
  return (
    <li className="flex items-center gap-3 p-3 sm:px-4">
      <div className="min-w-0 flex-1 space-y-1">
        <div className="flex h-6 min-w-0 items-center gap-2">{name}</div>
        <div className="text-muted-foreground flex h-5 min-w-0 items-center text-sm">
          {trigger}
        </div>
        <div className="text-muted-foreground flex h-4 min-w-0 items-center gap-1.5 text-xs">
          {timing}
        </div>
      </div>
      {control}
    </li>
  );
}

export function AutomationRow({
  automation,
  canOperate,
  busy,
  onToggle,
}: {
  automation: AutomationView;
  /** The creator or an admin: the switch pauses and resumes. */
  canOperate: boolean;
  busy: boolean;
  onToggle: (automation: AutomationView) => void;
}) {
  const { t, i18n } = useTranslations();
  const schedule = automation.triggers[0]?.schedule;
  const status = automationStatus(automation);
  const running = automation.enabled && !automation.pausedReason;
  return (
    <AutomationRowShell
      name={
        <>
          <Link
            to="/automations/$automationId"
            params={{ automationId: automation.id }}
            className="min-w-0 truncate font-medium hover:underline"
          >
            {automation.name}
          </Link>
          <AutomationStatusBadge automation={automation} />
        </>
      }
      trigger={
        <span className="truncate">
          {schedule
            ? describeSchedule(schedule, t, i18n.language)
            : t("agentAutomations.noTrigger")}
        </span>
      }
      timing={
        <>
          <span className="truncate">
            {t("agentAutomations.nextRun")}:{" "}
            {status === "active" ? (
              <InstantLabel value={automation.nextFireAt} direction="future" />
            ) : (
              "—"
            )}
          </span>
          <span aria-hidden>·</span>
          <span className="truncate">
            {t("agentAutomations.lastRun")}:{" "}
            {automation.lastFireAt ? (
              <InstantLabel value={automation.lastFireAt} direction="past" />
            ) : (
              t("agentAutomations.neverRun")
            )}
          </span>
          {automation.lastSessionId ? (
            <>
              <span aria-hidden>·</span>
              <Link
                to="/agents/$agentSessionId"
                params={{ agentSessionId: automation.lastSessionId }}
                className="shrink-0 underline-offset-2 hover:underline"
              >
                {t("agentAutomations.lastSession")}
              </Link>
            </>
          ) : null}
        </>
      }
      control={
        <Switch
          checked={running}
          disabled={!canOperate || busy}
          onCheckedChange={() => onToggle(automation)}
          aria-label={t(
            running
              ? "agentAutomations.pauseLabel"
              : "agentAutomations.resumeLabel",
            { name: automation.name },
          )}
        />
      }
    />
  );
}
