import { useId, useState } from "react";
import { toast } from "sonner";
import { Label } from "@/common/components/ui/label";
import { Switch } from "@/common/components/ui/switch";
import { useTranslations } from "@/common/hooks/use-translations";
import type { AgentEnvironmentInput } from "@/graphql/definitions";
import {
  githubCliErrorKey,
  type AgentEnvironmentView,
} from "@/features/agent-environment/lib/environment";

interface GithubCliSectionProps {
  saved: AgentEnvironmentView;
  update: (input: AgentEnvironmentInput) => Promise<AgentEnvironmentView>;
  canManage: boolean;
}

/**
 * Settings → Agents' GitHub CLI switch (w10/m524, a Devin security profile's
 * gh API access): on, a repo session's gh reaches its repository through
 * bex; off removes gh from every session in the workspace, running ones
 * included, while Git pushes and bex's pull request tools keep working. It
 * saves on toggle, as the available-agents switches do; members see it
 * disabled.
 */
export function GithubCliSection({
  saved,
  update,
  canManage,
}: GithubCliSectionProps) {
  const { t } = useTranslations();
  const id = `${useId()}-github-cli`;
  const helpId = `${id}-help`;
  const [errorKey, setErrorKey] = useState<string | null>(null);
  // The admin's choice while it saves. The saved answer reaches `saved`
  // through the Apollo cache, and a refused save leaves the stored value.
  const [pending, setPending] = useState<boolean | null>(null);
  const checked = pending ?? saved.githubCliEnabled;

  async function toggle(on: boolean) {
    setPending(on);
    setErrorKey(null);
    try {
      const next = await update({ githubCliEnabled: on });
      toast.success(
        t(
          next.githubCliEnabled
            ? "agentEnvironment.githubCliTurnedOn"
            : "agentEnvironment.githubCliTurnedOff",
        ),
      );
    } catch (err) {
      setErrorKey(githubCliErrorKey(err));
    } finally {
      setPending(null);
    }
  }

  return (
    <div className="space-y-3 border-t pt-6" data-testid="github-cli-section">
      <div className="flex max-w-xl items-start justify-between gap-4">
        <div className="min-w-0 space-y-1">
          <Label htmlFor={id}>{t("agentEnvironment.githubCliLabel")}</Label>
          <p id={helpId} className="text-muted-foreground text-sm">
            {t("agentEnvironment.githubCliHelp")}
          </p>
        </div>
        <Switch
          id={id}
          checked={checked}
          disabled={!canManage || pending !== null}
          aria-describedby={helpId}
          onCheckedChange={(on) => void toggle(on)}
        />
      </div>
      {errorKey ? (
        <p className="text-destructive text-sm" role="alert">
          {t(errorKey)}
        </p>
      ) : null}
    </div>
  );
}
