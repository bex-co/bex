import { useState } from "react";
import { toast } from "sonner";
import { Switch } from "@/common/components/ui/switch";
import { Label } from "@/common/components/ui/label";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import type { AgentEnvironmentInput } from "@/graphql/definitions";
import {
  AGENT_OPTIONS,
  type AgentOption,
} from "@/features/agent-sessions/lib/composer";
import type { AgentEnvironmentView } from "@/features/agent-environment/lib/environment";
import { AgentSectionHeading } from "@/features/agent-environment/components/agent-environment-card-skeleton";
import { PricingWithTerms } from "@/features/legal/components/agent-model-terms-link";

interface AvailableAgentsSectionProps {
  saved: AgentEnvironmentView;
  update: (input: AgentEnvironmentInput) => Promise<AgentEnvironmentView>;
  canManage: boolean;
}

/**
 * Settings → Agents' availability (w7/m392, Devin: "admins control which
 * agents are available"): one switch per agent profile. A profile whose model
 * provider may train on what its sessions send (agentDataUse, ADR108: Muse
 * Code) turns on only through a confirm that records the admin's
 * acknowledgement. Members see the switches disabled.
 */
export function AvailableAgentsSection({
  saved,
  update,
  canManage,
}: AvailableAgentsSectionProps) {
  const { t } = useTranslations();
  const [busy, setBusy] = useState(false);
  const [confirm, setConfirm] = useState<AgentOption | null>(null);
  const [failed, setFailed] = useState(false);
  const dataUse = new Map(saved.agentDataUse.map((use) => [use.agent, use]));
  const withAgent = (agent: AgentOption) => [...saved.enabledAgents, agent];

  async function write(input: AgentEnvironmentInput) {
    setBusy(true);
    setFailed(false);
    try {
      await update(input);
      toast.success(t("agentEnvironment.agentsSaved"));
    } catch {
      setFailed(true);
    } finally {
      setBusy(false);
      setConfirm(null);
    }
  }

  function toggle(agent: AgentOption, on: boolean) {
    const use = dataUse.get(agent);
    if (on && use && !use.acknowledgedAt) {
      setConfirm(agent);
      return;
    }
    void write({
      enabledAgents: on
        ? withAgent(agent)
        : saved.enabledAgents.filter((id) => id !== agent),
    });
  }

  return (
    <div
      className="space-y-3 border-t pt-6"
      data-testid="available-agents-section"
    >
      <AgentSectionHeading
        title={t("agentEnvironment.agentsTitle")}
        description={t("agentEnvironment.agentsDescription")}
      />
      <ul className="max-w-xl space-y-3">
        {AGENT_OPTIONS.map((agent) => {
          const id = `available-agent-${agent}`;
          return (
            <li key={agent} className="flex items-start justify-between gap-4">
              <div className="min-w-0 space-y-0.5">
                <Label htmlFor={id}>{t(`agentSessions.agent.${agent}`)}</Label>
                {dataUse.has(agent) ? (
                  <p className="text-muted-foreground text-xs">
                    {t("agentEnvironment.agentsDataUseNote")}
                  </p>
                ) : null}
              </div>
              <Switch
                id={id}
                checked={saved.enabledAgents.includes(agent)}
                disabled={!canManage || busy}
                onCheckedChange={(on) => toggle(agent, on)}
              />
            </li>
          );
        })}
      </ul>
      {failed ? (
        <p className="text-destructive text-sm" role="alert">
          {t("agentEnvironment.agentsSaveFailed")}
        </p>
      ) : null}
      <ConfirmDialog
        open={confirm != null}
        onOpenChange={(open) => {
          if (!open) setConfirm(null);
        }}
        title={t("agentEnvironment.agentsDataUseTitle", {
          agent: confirm ? t(`agentSessions.agent.${confirm}`) : "",
        })}
        description={
          <PricingWithTerms
            pricing={
              confirm ? t(`agentEnvironment.agentsDataUseBody.${confirm}`) : ""
            }
          />
        }
        confirmLabel={t("agentEnvironment.agentsDataUseConfirm")}
        destructive={false}
        closeOnConfirm={false}
        pending={busy}
        onConfirm={() =>
          confirm &&
          void write({
            enabledAgents: withAgent(confirm),
            acknowledgeAgentDataUse: [confirm],
          })
        }
      />
    </div>
  );
}
