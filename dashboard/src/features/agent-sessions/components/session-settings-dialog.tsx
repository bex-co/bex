import { useId, useState, type ReactNode } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/common/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { Label } from "@/common/components/ui/label";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAgentEnvironment } from "@/features/agent-environment/hooks/use-agent-environment";
import { useAgentAutomation } from "@/features/agent-automations/hooks/use-agent-automation";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { GithubCliPicker } from "@/features/agent-sessions/components/github-cli-picker";
import { useAgentSessionMutations } from "@/features/agent-sessions/hooks/use-agent-session-mutations";
import { useAgentSessionTree } from "@/features/agent-sessions/hooks/use-agent-session-tree";
import {
  AgentSessionError,
  agentSessionErrorMessage,
} from "@/features/agent-sessions/lib/errors";
import {
  githubCliChainOf,
  githubCliChoice,
  githubCliLock,
  inheritedSessionGithubCli,
  type GithubCliChoice,
  type SessionGithubCliChain,
} from "@/features/agent-sessions/lib/github-cli";
import type { AgentSessionView } from "@/features/agent-sessions/types";

type SettingsSession = Pick<
  AgentSessionView,
  "id" | "ownerId" | "automationId" | "parentSessionId" | "agentConfig"
>;

/**
 * A session's settings after it started (w10/m599), as Devin's session
 * security profile is "changed later from the session's settings": today its
 * own GitHub CLI binding. Every member sees what applies; only a workspace
 * admin changes it (w10/044), so a member's choices other than the stored one
 * are disabled with the reason, and nobody picks "on" under the workspace's
 * mandatory off. Inherit names what it resolves to through the session's
 * tree and automation, as bex-api resolves it. An off refuses the session's
 * next gh call; gh returns at its next turn. The caller mounts it only while
 * open, so each opening starts from the session's current binding.
 */
export function SessionSettingsDialog({
  session,
  onOpenChange,
}: {
  session: SettingsSession;
  onOpenChange: (open: boolean) => void;
}) {
  // A child inherits through the sessions above it, which the page's own
  // tree read already holds (cache-first); a top-level session reads none.
  const tree = useAgentSessionTree(session.parentSessionId ? session.id : "");
  const chain = githubCliChainOf(session, tree.loading ? null : tree.sessions);
  return (
    <SettingsDialog
      session={session}
      chain={chain}
      onOpenChange={onOpenChange}
    />
  );
}

/**
 * The automation binding a tree an automation started inherits through:
 * undefined until the automation is read, null for one that binds nothing
 * or was deleted.
 */
function AutomationBinding({
  automationId,
  children,
}: {
  automationId: string;
  children: (binding: boolean | null | undefined) => ReactNode;
}) {
  const { automation, loading } = useAgentAutomation(automationId);
  const binding = automation
    ? (automation.template.agentConfig.githubCli ?? null)
    : loading
      ? undefined
      : null;
  return children(binding);
}

function SettingsDialog({
  session,
  chain,
  onOpenChange,
}: {
  session: SettingsSession;
  chain: SessionGithubCliChain | null;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslations();
  const { setGithubCli } = useAgentSessionMutations();
  const { environment } = useAgentEnvironment(session.ownerId);
  const { canManage } = useCapabilities();
  const stored = githubCliChoice(session.agentConfig.githubCli);
  const [choice, setChoice] = useState<GithubCliChoice>(stored);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const fieldId = useId();
  const hintId = `${fieldId}-hint`;
  const errorId = `${fieldId}-error`;

  function handleOpenChange(next: boolean) {
    if (!saving) onOpenChange(next);
  }

  async function handleSave() {
    if (choice === stored) {
      onOpenChange(false);
      return;
    }
    setSaving(true);
    setError(null);
    try {
      // The mutation writes the server's view into the cache, so the header
      // and lists update without a refetch.
      await setGithubCli(session.id, choice);
      toast.success(t("agentSessions.settingsSaved"));
      onOpenChange(false);
    } catch (err) {
      setError(
        err instanceof AgentSessionError &&
          err.code === "AGENT_SESSION_GITHUB_CLI_LOCKED" &&
          err.params.reason === "admin_required"
          ? t("agentSessions.githubCliAdminOnly")
          : agentSessionErrorMessage(err, t),
      );
    } finally {
      setSaving(false);
    }
  }

  const picker = (automationBinding?: boolean | null) => (
    <GithubCliPicker
      id={fieldId}
      variant="field"
      value={choice}
      onChange={(next) => {
        setChoice(next);
        setError(null);
      }}
      inherited={inheritedSessionGithubCli(
        environment,
        chain,
        automationBinding,
      )}
      lockOf={(option) =>
        githubCliLock({ choice: option, stored, env: environment, canManage })
      }
      disabled={saving}
      invalid={error != null}
      describedBy={error ? `${hintId} ${errorId}` : hintId}
    />
  );

  return (
    <Dialog open onOpenChange={handleOpenChange}>
      <DialogContent data-testid="session-settings-dialog">
        <form
          className="space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            void handleSave();
          }}
        >
          <DialogHeader>
            <DialogTitle>{t("agentSessions.settingsTitle")}</DialogTitle>
            <DialogDescription>
              {t("agentSessions.settingsDescription")}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-1.5">
            <Label htmlFor={fieldId}>
              {t("agentEnvironment.githubCliLabel")}
            </Label>
            {chain?.rootAutomationId ? (
              <AutomationBinding automationId={chain.rootAutomationId}>
                {picker}
              </AutomationBinding>
            ) : (
              picker()
            )}
            <p id={hintId} className="text-muted-foreground text-xs">
              {t("agentSessions.settingsGithubCliHint")}
            </p>
            {error ? (
              <p id={errorId} role="alert" className="text-destructive text-sm">
                {error}
              </p>
            ) : null}
          </div>
          <DialogFooter>
            <Button type="submit" disabled={saving || choice === stored}>
              {saving ? <Loader2 className="animate-spin" /> : null}
              {t("agentSessions.settingsSave")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
