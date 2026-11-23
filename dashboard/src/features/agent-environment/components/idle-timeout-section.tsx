import { useId, useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { useTranslations } from "@/common/hooks/use-translations";
import type { AgentEnvironmentInput } from "@/graphql/definitions";
import {
  IDLE_TIMEOUT_MAX,
  IDLE_TIMEOUT_MIN,
  idleTimeoutErrorKey,
  parseIdleTimeout,
  type AgentEnvironmentView,
} from "@/features/agent-environment/lib/environment";
import { AgentSectionHeading } from "@/features/agent-environment/components/agent-environment-card-skeleton";

interface IdleTimeoutSectionProps {
  saved: AgentEnvironmentView;
  update: (input: AgentEnvironmentInput) => Promise<AgentEnvironmentView>;
  canManage: boolean;
}

const BOUNDS = { min: IDLE_TIMEOUT_MIN, max: IDLE_TIMEOUT_MAX };

/**
 * Settings → Agents' idle timeout (w6/m563): how long the sandbox of a
 * session a person started stays up after its turn ends or its last editor
 * disconnects, 5 to 120 minutes, as Devin's Enterprise customers can choose.
 * The card draws it only on a workspace whose plan allows it (Enterprise);
 * every other plan keeps the platform's 5 minutes. It saves on its own, as
 * the spend cap does; members see it disabled.
 */
export function IdleTimeoutSection({
  saved,
  update,
  canManage,
}: IdleTimeoutSectionProps) {
  const { t } = useTranslations();
  const inputId = `${useId()}-idle-timeout`;
  const helpId = `${inputId}-help`;
  const errorId = `${inputId}-error`;
  const [draft, setDraft] = useState(() => String(saved.idleTimeoutMinutes));
  const [base, setBase] = useState(saved);
  if (base !== saved) {
    setBase(saved);
    setDraft(String(saved.idleTimeoutMinutes));
  }
  const [busy, setBusy] = useState(false);
  const [errorKey, setErrorKey] = useState<string | null>(null);

  const value = parseIdleTimeout(draft);
  const invalid = draft.trim() !== "" && value == null;
  const shownError = invalid ? "agentEnvironment.idleTimeoutInvalid" : errorKey;

  async function save(minutes: number) {
    setErrorKey(null);
    setBusy(true);
    try {
      await update({ idleTimeoutMinutes: minutes });
      toast.success(t("agentEnvironment.idleTimeoutSaved", { minutes }));
    } catch (err) {
      setErrorKey(idleTimeoutErrorKey(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-3 border-t pt-6" data-testid="idle-timeout-section">
      <AgentSectionHeading
        title={t("agentEnvironment.idleTimeoutTitle")}
        description={t("agentEnvironment.idleTimeoutDescription")}
      />
      <div className="space-y-2">
        <Label htmlFor={inputId}>
          {t("agentEnvironment.idleTimeoutLabel")}
        </Label>
        <div className="flex max-w-xl flex-wrap gap-2">
          <Input
            id={inputId}
            type="number"
            inputMode="numeric"
            min={IDLE_TIMEOUT_MIN}
            max={IDLE_TIMEOUT_MAX}
            step={1}
            value={draft}
            onChange={(e) => {
              setDraft(e.target.value);
              setErrorKey(null);
            }}
            disabled={!canManage}
            aria-invalid={shownError != null}
            aria-describedby={shownError ? `${helpId} ${errorId}` : helpId}
            className="max-w-40 tabular-nums"
          />
          <Button
            type="button"
            variant="outline"
            className="min-w-24"
            disabled={
              !canManage ||
              value == null ||
              value === saved.idleTimeoutMinutes ||
              busy
            }
            onClick={() => value != null && void save(value)}
          >
            {busy ? <Loader2 className="animate-spin" /> : null}
            {t("agentEnvironment.idleTimeoutSave")}
          </Button>
        </div>
        <p id={helpId} className="text-muted-foreground text-xs">
          {t("agentEnvironment.idleTimeoutHelp", BOUNDS)}
        </p>
        {shownError ? (
          <p id={errorId} className="text-destructive text-sm" role="alert">
            {t(shownError, BOUNDS)}
          </p>
        ) : null}
      </div>
    </div>
  );
}
