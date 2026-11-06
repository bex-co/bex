import { useState } from "react";
import { Loader2, Trash2 } from "lucide-react";
import { toast } from "sonner";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useAgentSecrets } from "@/features/agent-environment/hooks/use-agent-secrets";
import { AgentSecretsCardSkeleton } from "@/features/agent-environment/components/agent-secrets-card-skeleton";
import {
  secretErrorKey,
  secretNameProblem,
  secretValueProblem,
} from "@/features/agent-environment/lib/secrets";
import { SECRET_INPUT_PROPS } from "@/common/lib/secret-input";

/**
 * Settings → Agent secrets (ADR098 D8): the workspace's agent secrets, which a
 * session can be given as env vars. Write-only: the list shows names, a value
 * is typed into a masked field that is emptied after each save, and nothing
 * ever reads one back. Members see the names; only admins set or delete.
 */
export function AgentSecretsCard({ ownerId }: { ownerId: string }) {
  const { t } = useTranslations();
  const { canManage } = useCapabilities();
  const { names, loading, error, refetch, set, remove, saving } =
    useAgentSecrets(ownerId);
  const [name, setName] = useState("");
  const [value, setValue] = useState("");
  const [errorKey, setErrorKey] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);

  if (loading) return <AgentSecretsCardSkeleton />;

  const trimmed = name.trim();
  const nameProblem = trimmed === "" ? null : secretNameProblem(trimmed);
  const valueProblem = value === "" ? null : secretValueProblem(value);
  const canSave =
    canManage &&
    !saving &&
    trimmed !== "" &&
    value !== "" &&
    nameProblem == null &&
    valueProblem == null;
  const shownError = nameProblem ?? valueProblem ?? errorKey;

  async function save() {
    setErrorKey(null);
    try {
      await set(trimmed, value);
      toast.success(t("agentEnvironment.secrets.saved", { name: trimmed }));
      setName("");
      setValue("");
    } catch (err) {
      setErrorKey(secretErrorKey(err));
    }
  }

  async function confirmDelete(target: string) {
    setErrorKey(null);
    try {
      await remove(target);
      toast.success(t("agentEnvironment.secrets.deleted", { name: target }));
    } catch (err) {
      setErrorKey(secretErrorKey(err));
    } finally {
      setDeleting(null);
    }
  }

  return (
    <Card data-testid="agent-secrets-card">
      <CardHeader>
        <CardTitle>{t("agentEnvironment.secrets.title")}</CardTitle>
        <CardDescription>
          {t("agentEnvironment.secrets.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {error && names.length === 0 ? (
          <Alert variant="destructive">
            <AlertDescription className="flex flex-wrap items-center gap-3">
              <span>{t("agentEnvironment.secrets.loadFailed")}</span>
              <Button
                size="sm"
                variant="outline"
                onClick={() => void refetch()}
              >
                {t("agentEnvironment.retry")}
              </Button>
            </AlertDescription>
          </Alert>
        ) : names.length === 0 ? (
          <p className="text-muted-foreground flex h-9 items-center text-sm">
            {t("agentEnvironment.secrets.empty")}
          </p>
        ) : (
          <ul
            className="divide-y rounded-md border"
            data-testid="agent-secret-names"
          >
            {names.map((secret) => (
              <li
                key={secret}
                className="flex h-9 items-center justify-between gap-2 pr-1 pl-3"
              >
                <span className="truncate font-mono text-sm">{secret}</span>
                {canManage ? (
                  <ConfirmDialog
                    open={deleting === secret}
                    onOpenChange={(open) => setDeleting(open ? secret : null)}
                    trigger={
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="size-7"
                        aria-label={t("agentEnvironment.secrets.deleteLabel", {
                          name: secret,
                        })}
                        disabled={saving}
                      >
                        <Trash2 className="size-4" />
                      </Button>
                    }
                    title={t("agentEnvironment.secrets.deleteTitle", {
                      name: secret,
                    })}
                    description={t("agentEnvironment.secrets.deleteBody")}
                    confirmLabel={t("agentEnvironment.secrets.deleteConfirm")}
                    destructive
                    closeOnConfirm={false}
                    pending={saving && deleting === secret}
                    onConfirm={() => void confirmDelete(secret)}
                  />
                ) : null}
              </li>
            ))}
          </ul>
        )}
        {/* Rendered for everyone, disabled for non-admins, so the loaded card
            keeps its skeleton's geometry. */}
        <form
          className="space-y-2 border-t pt-4"
          onSubmit={(e) => {
            e.preventDefault();
            if (canSave) void save();
          }}
        >
          <div className="flex max-w-2xl flex-wrap gap-2">
            <Input
              value={name}
              onChange={(e) => {
                setName(e.target.value);
                setErrorKey(null);
              }}
              placeholder={t("agentEnvironment.secrets.namePlaceholder")}
              aria-label={t("agentEnvironment.secrets.nameLabel")}
              aria-invalid={nameProblem != null}
              disabled={!canManage}
              spellCheck={false}
              autoCapitalize="off"
              autoComplete="off"
              className="w-48 font-mono"
            />
            <Input
              type="password"
              value={value}
              onChange={(e) => {
                setValue(e.target.value);
                setErrorKey(null);
              }}
              placeholder={t("agentEnvironment.secrets.valuePlaceholder")}
              aria-label={t("agentEnvironment.secrets.valueLabel")}
              aria-invalid={valueProblem != null}
              disabled={!canManage}
              {...SECRET_INPUT_PROPS}
              className="min-w-48 flex-1 font-mono"
            />
            <Button type="submit" variant="outline" disabled={!canSave}>
              {saving && deleting == null ? (
                <Loader2 className="animate-spin" />
              ) : null}
              {t("agentEnvironment.secrets.save")}
            </Button>
          </div>
          <p className="text-muted-foreground text-xs">
            {canManage
              ? t("agentEnvironment.secrets.help")
              : t("agentEnvironment.secrets.readOnly")}
          </p>
          {shownError ? (
            <p className="text-destructive text-sm" role="alert">
              {t(shownError)}
            </p>
          ) : null}
        </form>
      </CardContent>
    </Card>
  );
}
