import { useId, useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  AgentModelSource,
  type AgentEnvironmentInput,
} from "@/graphql/definitions";
import {
  modelKeyErrorKey,
  normalizeModelKey,
  type AgentEnvironmentView,
} from "@/features/agent-environment/lib/environment";

const SOURCE_KEYS: Record<AgentModelSource, string> = {
  [AgentModelSource.Platform]: "agentEnvironment.modelSourcePlatform",
  [AgentModelSource.Byo]: "agentEnvironment.modelSourceByo",
  [AgentModelSource.None]: "agentEnvironment.modelSourceNone",
};

interface ModelKeySectionProps {
  saved: AgentEnvironmentView;
  update: (input: AgentEnvironmentInput) => Promise<AgentEnvironmentView>;
  canManage: boolean;
}

/**
 * Settings → Agents' model source (ADR091 D7): whether claude sessions run
 * on the bex-provided model (billed at provider cost + 30%) or the
 * workspace's own provider key. Admins set, rotate or remove the key through
 * a write-only field that is never prefilled and is emptied after each save;
 * everyone else sees the status only.
 */
export function ModelKeySection({
  saved,
  update,
  canManage,
}: ModelKeySectionProps) {
  const { t } = useTranslations();
  const inputId = `${useId()}-model-key`;
  const [keyText, setKeyText] = useState("");
  const [busy, setBusy] = useState<"save" | "clear" | null>(null);
  const [errorKey, setErrorKey] = useState<string | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);

  const key = normalizeModelKey(keyText);
  const invalid = keyText.trim() !== "" && key == null;
  const shownError = invalid ? "agentEnvironment.modelKeyInvalid" : errorKey;

  async function write(input: AgentEnvironmentInput, doneKey: string) {
    setErrorKey(null);
    setBusy(input.clearModelApiKey ? "clear" : "save");
    try {
      await update(input);
      setKeyText("");
      setConfirmOpen(false);
      toast.success(t(doneKey));
    } catch (err) {
      setErrorKey(modelKeyErrorKey(err));
      setConfirmOpen(false);
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="space-y-3 border-t pt-6" data-testid="model-key-section">
      <div className="space-y-1">
        <h3 className="text-sm font-medium">
          {t("agentEnvironment.modelKeyTitle")}
        </h3>
        <p className="text-muted-foreground text-sm">
          {t("agentEnvironment.modelKeyDescription")}
        </p>
      </div>
      <p className="text-sm" data-testid="model-source">
        <span className="text-muted-foreground">
          {t("agentEnvironment.modelSourceLabel")}{" "}
        </span>
        <span className="font-medium">{t(SOURCE_KEYS[saved.modelSource])}</span>
      </p>
      {/* Rendered for everyone, disabled for non-admins (the card's read-only
          note explains why), so the loaded card keeps its skeleton's geometry. */}
      <div className="space-y-2">
        <Label htmlFor={inputId}>
          {saved.modelKeySet
            ? t("agentEnvironment.modelKeyReplaceLabel")
            : t("agentEnvironment.modelKeyLabel")}
        </Label>
        <div className="flex max-w-xl flex-wrap gap-2">
          <Input
            id={inputId}
            type="password"
            value={keyText}
            onChange={(e) => {
              setKeyText(e.target.value);
              setErrorKey(null);
            }}
            placeholder={
              saved.modelKeySet
                ? t("agentEnvironment.modelKeyPlaceholderSet")
                : t("agentEnvironment.modelKeyPlaceholder")
            }
            disabled={!canManage}
            // A provider key is not a login password: keep browsers and
            // password managers from offering, saving or filling it.
            autoComplete="new-password"
            data-1p-ignore
            data-lpignore="true"
            data-bwignore
            spellCheck={false}
            autoCapitalize="off"
            aria-invalid={shownError != null}
            aria-describedby={`${inputId}-help`}
            className="max-w-sm flex-1 font-mono"
          />
          <Button
            type="button"
            variant="outline"
            disabled={!canManage || key == null || busy != null}
            onClick={() =>
              key != null &&
              void write({ modelApiKey: key }, "agentEnvironment.modelKeySaved")
            }
          >
            {busy === "save" ? <Loader2 className="animate-spin" /> : null}
            {t("agentEnvironment.modelKeySave")}
          </Button>
          {saved.modelKeySet ? (
            <ConfirmDialog
              open={confirmOpen}
              onOpenChange={setConfirmOpen}
              trigger={
                <Button
                  type="button"
                  variant="ghost"
                  disabled={!canManage || busy != null}
                >
                  {t("agentEnvironment.modelKeyClear")}
                </Button>
              }
              title={t("agentEnvironment.modelKeyClearTitle")}
              description={t("agentEnvironment.modelKeyClearBody")}
              confirmLabel={t("agentEnvironment.modelKeyClearConfirm")}
              destructive
              closeOnConfirm={false}
              pending={busy === "clear"}
              onConfirm={() =>
                void write(
                  { clearModelApiKey: true },
                  "agentEnvironment.modelKeyCleared",
                )
              }
            />
          ) : null}
        </div>
        <p id={`${inputId}-help`} className="text-muted-foreground text-xs">
          {t("agentEnvironment.modelKeyHelp")}
        </p>
        {shownError ? (
          <p className="text-destructive text-sm" role="alert">
            {t(shownError)}
          </p>
        ) : null}
      </div>
    </div>
  );
}
