import { Link } from "@tanstack/react-router";
import { Badge } from "@/common/components/ui/badge";
import { Skeleton } from "@/common/components/ui/skeleton";
import { MetadataList } from "@/common/components/metadata-list";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatPreciseUsd } from "@/features/agent-sessions/lib/spend-limit";
import { useWorkspaceSubjectLabel } from "@/common/hooks/use-workspace-subject-label";
import { InstantLabel } from "@/features/agent-automations/components/instant-label";
import { AutomationStatusBadge } from "@/features/agent-automations/components/automation-status";
import type { AutomationView } from "@/features/agent-automations/hooks/use-agent-automations";
import {
  automationStatus,
  emailNotificationLabel,
} from "@/features/agent-automations/lib/display";
import { describeSchedule } from "@/features/agent-automations/lib/schedule";
import { useAgentEnvironment } from "@/features/agent-environment/hooks/use-agent-environment";
import {
  githubCliChoice,
  githubCliChoiceLabel,
  inheritedGithubCli,
} from "@/features/agent-sessions/lib/github-cli";

/** The settings rows, in order. The skeleton and the ready card share it. */
const ROW_LABELS = [
  "status",
  "schedule",
  "nextRun",
  "lastRun",
  "createdBy",
  "repo",
  "agent",
  "githubCli",
  "spendLimit",
  "emailNotifications",
  "secrets",
  "skills",
] as const;

// The gh row reads as the control it shows does (w10/m561).
const rowLabelKey = (key: (typeof ROW_LABELS)[number]) =>
  key === "githubCli"
    ? "agentEnvironment.githubCliLabel"
    : `agentAutomations.row.${key}`;

function CreatedBy({ subject }: { subject: string }) {
  return <>{useWorkspaceSubjectLabel(subject)}</>;
}

/**
 * The automation's settings (ADR101 D9) as the shared MetadataList card, so
 * pending (`automation` null, a bar per value) and ready are literally the
 * same card.
 */
export function AutomationSettingsCard({
  automation,
}: {
  automation: AutomationView | null;
}) {
  const { t, i18n } = useTranslations();
  const tpl = automation?.template;
  const schedule = automation?.triggers[0]?.schedule;
  // What an inheriting automation's sessions get (w10/m561): the workspace's
  // automations default, else its GitHub CLI switch.
  const { environment } = useAgentEnvironment(automation?.ownerId ?? null);
  const bar = <Skeleton className="h-4 w-24" />;
  const values: Record<(typeof ROW_LABELS)[number], React.ReactNode> =
    automation && tpl
      ? {
          status:
            automationStatus(automation) === "active" ? (
              <Badge variant="success">{t("agentAutomations.active")}</Badge>
            ) : (
              <AutomationStatusBadge automation={automation} />
            ),
          schedule: schedule
            ? describeSchedule(schedule, t, i18n.language)
            : t("agentAutomations.noTrigger"),
          nextRun:
            automationStatus(automation) === "active" ? (
              <InstantLabel value={automation.nextFireAt} direction="future" />
            ) : (
              "—"
            ),
          lastRun: automation.lastFireAt ? (
            <span className="inline-flex flex-wrap items-center justify-end gap-x-2">
              <InstantLabel value={automation.lastFireAt} direction="past" />
              {automation.lastSessionId ? (
                <Link
                  to="/agents/$agentSessionId"
                  params={{ agentSessionId: automation.lastSessionId }}
                  className="underline underline-offset-2"
                >
                  {t("agentAutomations.lastSession")}
                </Link>
              ) : null}
            </span>
          ) : (
            t("agentAutomations.neverRun")
          ),
          createdBy: <CreatedBy subject={automation.createdBy} />,
          repo: tpl.repo || t("agentAutomations.noRepo"),
          agent: t(`agentSessions.agent.${tpl.agentConfig.agent}`),
          githubCli: githubCliChoiceLabel(
            githubCliChoice(tpl.agentConfig.githubCli),
            inheritedGithubCli(environment, "automation"),
            t,
          ),
          spendLimit:
            tpl.maxSpendUSD != null
              ? formatPreciseUsd(String(tpl.maxSpendUSD), i18n.language)
              : t("agentAutomations.noSpendLimit"),
          emailNotifications: emailNotificationLabel(
            automation.notifications.email,
            t,
          ),
          secrets: tpl.secretNames?.length
            ? tpl.secretNames.join(", ")
            : t("agentAutomations.none"),
          skills: tpl.skills?.length
            ? tpl.skills.map((s) => s.name).join(", ")
            : t("agentAutomations.none"),
        }
      : {
          status: bar,
          schedule: bar,
          nextRun: bar,
          lastRun: bar,
          createdBy: bar,
          repo: bar,
          agent: bar,
          githubCli: bar,
          spendLimit: bar,
          emailNotifications: bar,
          secrets: bar,
          skills: bar,
        };
  return (
    <MetadataList
      title={t("agentAutomations.settingsTitle")}
      rows={ROW_LABELS.map((key) => ({
        label: t(rowLabelKey(key)),
        value: values[key],
      }))}
    />
  );
}
