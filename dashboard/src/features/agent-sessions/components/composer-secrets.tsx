import { useId, useState, type KeyboardEvent } from "react";
import { Link } from "@tanstack/react-router";
import { KeyRound, Loader2, X } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Checkbox } from "@/common/components/ui/checkbox";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/common/components/ui/popover";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAgentSecrets } from "@/features/agent-environment/hooks/use-agent-secrets";
import {
  MAX_SESSION_SECRETS,
  secretAppliesByScope,
  secretNameProblem,
  secretValueProblem,
  type AgentSecretView,
} from "@/features/agent-environment/lib/secrets";
import {
  composerSecretNames,
  type ComposerSecretsValue,
} from "@/features/agent-sessions/lib/composer-secrets";
import { SECRET_INPUT_PROPS } from "@/common/lib/secret-input";

interface ComposerSecretsProps {
  ownerId: string | null;
  value: ComposerSecretsValue;
  onChange: (value: ComposerSecretsValue) => void;
  disabled?: boolean;
  /**
   * The session's repository (owner/name), or none for a chat-only session:
   * secrets scoped to it are included without being picked (ADR098 D9).
   */
  repo?: string | null;
  /**
   * Offer only the workspace's stored secrets. An automation template (ADR101
   * D5) carries names, never inline values, so its form hides the inline half.
   */
  workspaceOnly?: boolean;
}

/**
 * The composer's Secrets control (ADR098 D8), Devin's secret_ids plus
 * session_secrets: pick workspace agent secrets and add inline ones the agent
 * reads as env vars. A fixed-size icon button on the toolbar, so the prompt
 * box keeps its skeleton geometry; the workspace list loads only when opened.
 *
 * Scoped secrets (ADR098 D9) are listed as included, not picked: a
 * workspace-scoped one always, a repo-scoped one when the session's
 * repository matches. The server applies them itself, so they never join
 * `secretNames`; only the explicit picks do.
 */
export function ComposerSecrets({
  ownerId,
  value,
  onChange,
  disabled,
  repo = null,
  workspaceOnly = false,
}: ComposerSecretsProps) {
  const { t } = useTranslations();
  const count = composerSecretNames(value).length;
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant={count > 0 ? "secondary" : "ghost"}
          size="icon"
          className="size-8"
          disabled={disabled}
          aria-label={
            count > 0
              ? t("agentSessions.secretsButtonCount", { count })
              : t("agentSessions.secretsButton")
          }
          title={t("agentSessions.secretsButton")}
          data-testid="agent-composer-secrets"
        >
          <KeyRound className="size-4" />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align="start"
        className="w-80 space-y-4"
        aria-label={t("agentSessions.secretsButton")}
      >
        <SecretsPanel
          ownerId={ownerId}
          value={value}
          onChange={onChange}
          repo={repo}
          workspaceOnly={workspaceOnly}
        />
      </PopoverContent>
    </Popover>
  );
}

function SecretsPanel({
  ownerId,
  value,
  onChange,
  repo,
  workspaceOnly = false,
}: Omit<ComposerSecretsProps, "disabled">) {
  const { t } = useTranslations();
  const ids = useId();
  const { secrets, loading, error } = useAgentSecrets(ownerId);
  const [name, setName] = useState("");
  const [secretValue, setSecretValue] = useState("");
  const full = composerSecretNames(value).length >= MAX_SESSION_SECRETS;

  const trimmed = name.trim();
  const duplicate = value.secrets.some((s) => s.name === trimmed);
  const problem =
    trimmed === ""
      ? null
      : duplicate
        ? "agentSessions.secretsDuplicate"
        : secretNameProblem(trimmed);
  const valueProblem =
    secretValue === "" ? null : secretValueProblem(secretValue);
  const canAdd =
    !full &&
    trimmed !== "" &&
    secretValue !== "" &&
    problem == null &&
    valueProblem == null;

  /** Why a scoped secret is or isn't included, beside its name. */
  function scopeHint(secret: AgentSecretView, included: boolean): string {
    if (!included) {
      return t("agentSessions.secretsScopeOtherRepo", {
        repo: secret.repo ?? "",
      });
    }
    return secret.scope === "workspace"
      ? t("agentEnvironment.secrets.scopeWorkspace")
      : t("agentSessions.secretsScopeRepo");
  }

  function toggle(secret: string, checked: boolean) {
    const next = checked
      ? [...value.secretNames, secret]
      : value.secretNames.filter((n) => n !== secret);
    onChange({ ...value, secretNames: next });
  }

  function add() {
    if (!canAdd) return;
    onChange({
      ...value,
      secrets: [...value.secrets, { name: trimmed, value: secretValue }],
    });
    setName("");
    setSecretValue("");
  }

  // Enter adds the secret instead of submitting the composer. Not a <form>:
  // the popover portal would bubble its submit to the composer's own form.
  function addOnEnter(e: KeyboardEvent) {
    if (e.key !== "Enter") return;
    e.preventDefault();
    add();
  }

  return (
    <>
      <div className="space-y-1">
        <p className="text-sm font-medium">{t("agentSessions.secretsTitle")}</p>
        <p className="text-muted-foreground text-xs">
          {t("agentSessions.secretsDescription")}
        </p>
      </div>
      <div className="space-y-2" data-testid="agent-composer-workspace-secrets">
        <p className="text-xs font-medium">
          {t("agentSessions.secretsWorkspace")}
        </p>
        {loading ? (
          <Loader2 className="text-muted-foreground size-4 animate-spin" />
        ) : error ? (
          <p className="text-destructive text-xs" role="alert">
            {t("agentSessions.secretsWorkspaceLoadFailed")}
          </p>
        ) : secrets.length === 0 ? (
          <p className="text-muted-foreground text-xs">
            {t("agentSessions.secretsWorkspaceEmpty")}{" "}
            <Link
              to="/workspace/settings"
              hash="agents"
              className="underline underline-offset-4"
            >
              {t("agentSessions.secretsManage")}
            </Link>
          </p>
        ) : (
          <ul className="max-h-40 space-y-1.5 overflow-auto">
            {secrets.map((secret) => {
              const id = `${ids}-${secret.name}`;
              const picked = value.secretNames.includes(secret.name);
              const scoped = secretAppliesByScope(secret, repo);
              // A repo-scoped secret off this repository can't be picked;
              // one an automation already names stays removable.
              const offRepo = secret.scope === "repo" && !scoped && !picked;
              return (
                <li key={secret.name} className="flex items-center gap-2">
                  <Checkbox
                    id={id}
                    checked={scoped || picked}
                    disabled={scoped || offRepo || (!picked && full)}
                    onCheckedChange={(c) => toggle(secret.name, c === true)}
                  />
                  <Label
                    htmlFor={id}
                    className="min-w-0 truncate font-mono text-xs"
                  >
                    {secret.name}
                  </Label>
                  {scoped || offRepo ? (
                    <span
                      className="text-muted-foreground ml-auto max-w-[55%] truncate text-xs"
                      data-testid="agent-composer-secret-scope"
                    >
                      {scopeHint(secret, scoped)}
                    </span>
                  ) : null}
                </li>
              );
            })}
          </ul>
        )}
      </div>
      {workspaceOnly ? null : (
        <div className="space-y-2 border-t pt-3">
          <p className="text-xs font-medium">
            {t("agentSessions.secretsInline")}
          </p>
          {value.secrets.length > 0 ? (
            <ul
              className="space-y-1"
              data-testid="agent-composer-inline-secrets"
            >
              {value.secrets.map((s) => (
                <li
                  key={s.name}
                  className="flex items-center justify-between gap-2 text-xs"
                >
                  <span className="truncate font-mono">{s.name}</span>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="size-6"
                    aria-label={t("agentSessions.secretsRemove", {
                      name: s.name,
                    })}
                    onClick={() =>
                      onChange({
                        ...value,
                        secrets: value.secrets.filter((x) => x.name !== s.name),
                      })
                    }
                  >
                    <X className="size-3.5" />
                  </Button>
                </li>
              ))}
            </ul>
          ) : null}
          <div className="flex gap-1.5">
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={addOnEnter}
              placeholder={t("agentSessions.secretsNamePlaceholder")}
              aria-label={t("agentSessions.secretsNameLabel")}
              aria-invalid={problem != null}
              spellCheck={false}
              autoCapitalize="off"
              autoComplete="off"
              className="h-8 w-28 font-mono text-xs"
            />
            <Input
              type="password"
              value={secretValue}
              onChange={(e) => setSecretValue(e.target.value)}
              onKeyDown={addOnEnter}
              placeholder={t("agentSessions.secretsValuePlaceholder")}
              aria-label={t("agentSessions.secretsValueLabel")}
              aria-invalid={valueProblem != null}
              {...SECRET_INPUT_PROPS}
              className="h-8 min-w-0 flex-1 font-mono text-xs"
            />
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="h-8"
              disabled={!canAdd}
              onClick={add}
            >
              {t("agentSessions.secretsAdd")}
            </Button>
          </div>
          <p
            className={
              (problem ?? valueProblem)
                ? "text-destructive text-xs"
                : "text-muted-foreground text-xs"
            }
            role={(problem ?? valueProblem) ? "alert" : undefined}
          >
            {t(
              problem ??
                valueProblem ??
                (full
                  ? "agentSessions.secretsFull"
                  : "agentSessions.secretsHint"),
            )}
          </p>
        </div>
      )}
    </>
  );
}
