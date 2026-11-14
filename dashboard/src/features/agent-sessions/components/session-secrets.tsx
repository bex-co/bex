import { KeyRound } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/common/components/ui/popover";
import { useTranslations } from "@/common/hooks/use-translations";
import { ownValue } from "@/common/lib/own-value";
import { useAgentSessionAppliedSecrets } from "@/features/agent-sessions/hooks/use-agent-session-applied-secrets";
import type { AgentSessionView } from "@/features/agent-sessions/types";

/** Why a turn received a secret (ADR098 D9), as the header names it. */
const SOURCE_KEYS: Record<string, string> = {
  inline: "agentSessions.appliedSecretSource.inline",
  explicit: "agentSessions.appliedSecretSource.explicit",
  repo: "agentSessions.appliedSecretSource.repo",
  workspace: "agentSessions.appliedSecretSource.workspace",
};

/**
 * The session's secrets in the detail header's meta row (ADR098). Once a turn
 * has run, the count is of what the latest turn received, picked or by scope,
 * and the popover lists each name with its source; never a value. Before
 * that, it counts the secrets the session attached, as it always has.
 */
export function SessionSecrets({ session }: { session: AgentSessionView }) {
  const { t } = useTranslations();
  const applied = useAgentSessionAppliedSecrets(session.id);

  if (applied.length === 0) {
    if (session.secretNames.length === 0) return null;
    return (
      <span
        className="inline-flex items-center gap-1"
        title={session.secretNames.join(", ")}
        data-testid="session-secret-names"
      >
        <KeyRound className="size-3" />
        {t("agentSessions.metaSecrets", { count: session.secretNames.length })}
      </span>
    );
  }

  const rows = applied.map((secret) => {
    const key = ownValue(SOURCE_KEYS, secret.source);
    return { name: secret.name, source: key ? t(key) : secret.source };
  });
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="link"
          className="text-muted-foreground h-auto gap-1 p-0 text-xs font-normal"
          title={rows.map((row) => `${row.name} · ${row.source}`).join(", ")}
          data-testid="session-applied-secrets"
        >
          <KeyRound className="size-3" />
          {t("agentSessions.metaSecrets", { count: applied.length })}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-72 space-y-2 text-sm">
        <p className="font-medium">{t("agentSessions.appliedSecretsTitle")}</p>
        <ul
          className="space-y-1 text-xs"
          aria-label={t("agentSessions.appliedSecretsTitle")}
        >
          {rows.map((row) => (
            <li key={row.name} className="flex min-w-0 gap-1">
              <span className="truncate font-mono">{row.name}</span>
              <span className="text-muted-foreground shrink-0">
                · {row.source}
              </span>
            </li>
          ))}
        </ul>
      </PopoverContent>
    </Popover>
  );
}
