import { useRef, useState } from "react";
import { Loader2, Pencil, Trash2 } from "lucide-react";
import { toast } from "sonner";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { ConnectedRepoCombobox } from "@/features/services/components/connected-repo-combobox";
import { useAgentSecrets } from "@/features/agent-environment/hooks/use-agent-secrets";
import { AgentSecretsCardSkeleton } from "@/features/agent-environment/components/agent-secrets-card-skeleton";
import {
  AGENT_SECRET_SCOPES,
  normalizeSecretRepo,
  secretErrorKey,
  secretNameProblem,
  secretValueProblem,
  type AgentSecretScope,
  type AgentSecretView,
} from "@/features/agent-environment/lib/secrets";
import { SECRET_INPUT_PROPS } from "@/common/lib/secret-input";

/** The scope picker's option labels (ADR098 D9). */
const SCOPE_OPTION_KEYS: Record<AgentSecretScope, string> = {
  workspace: "agentEnvironment.secrets.scopeWorkspace",
  repo: "agentEnvironment.secrets.scopeRepo",
  explicit: "agentEnvironment.secrets.scopeExplicit",
};

/** A scope the admin picked in the form, overriding the stored or default one. */
interface ScopeDraft {
  scope: AgentSecretScope;
  repo: string;
}

/**
 * Settings → Agent secrets (ADR098 D8, D9): the workspace's agent secrets,
 * which sessions receive as env vars. Each one's scope says which: every
 * session, sessions on one repository, or only sessions that pick it.
 * Write-only: the list shows names and scopes, a value is typed into a masked
 * field that is emptied after each save, and nothing ever reads one back.
 * Saving an existing name with an empty value changes only its scope. Members
 * see the list; only admins set, rescope or delete.
 */
export function AgentSecretsCard({ ownerId }: { ownerId: string }) {
  const { t } = useTranslations();
  const { canManage } = useCapabilities();
  const { secrets, loading, error, refetch, set, remove, saving } =
    useAgentSecrets(ownerId);
  const [name, setName] = useState("");
  const [value, setValue] = useState("");
  const [draft, setDraft] = useState<ScopeDraft | null>(null);
  const [errorKey, setErrorKey] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);
  const scopeTrigger = useRef<HTMLButtonElement>(null);

  if (loading) return <AgentSecretsCardSkeleton />;

  const trimmed = name.trim();
  // Typing an existing name edits that secret: its scope shows until the
  // admin picks another, and an empty value keeps the stored one.
  const existing = secrets.find((secret) => secret.name === trimmed) ?? null;
  const scope = draft?.scope ?? existing?.scope ?? "workspace";
  const repo = draft?.repo ?? existing?.repo ?? "";
  const normalizedRepo = scope === "repo" ? normalizeSecretRepo(repo) : null;
  const scopeChanged =
    existing != null &&
    (scope !== existing.scope ||
      (scope === "repo" && normalizedRepo !== existing.repo));

  const nameProblem = trimmed === "" ? null : secretNameProblem(trimmed);
  const valueProblem = value === "" ? null : secretValueProblem(value);
  const repoProblem =
    scope === "repo" && repo.trim() !== "" && normalizedRepo == null
      ? "agentEnvironment.secrets.repoInvalid"
      : null;
  const canSave =
    canManage &&
    !saving &&
    trimmed !== "" &&
    (value !== "" || scopeChanged) &&
    (scope !== "repo" || normalizedRepo != null) &&
    nameProblem == null &&
    valueProblem == null;
  const shownError = nameProblem ?? valueProblem ?? repoProblem ?? errorKey;

  function changeScope(next: ScopeDraft) {
    setDraft(next);
    setErrorKey(null);
  }

  /** Loads a listed secret into the form to rotate or rescope it. */
  function edit(secret: AgentSecretView) {
    setName(secret.name);
    setValue("");
    setDraft(null);
    setErrorKey(null);
    scopeTrigger.current?.focus();
  }

  async function save() {
    setErrorKey(null);
    try {
      await set(trimmed, {
        value: value === "" ? undefined : value,
        // Only a picked scope is sent: none keeps an existing secret's, and
        // gives a new one the server's default, workspace.
        scope: draft ? scope : undefined,
        repo: draft && scope === "repo" ? repo.trim() : undefined,
      });
      toast.success(t("agentEnvironment.secrets.saved", { name: trimmed }));
      setName("");
      setValue("");
      setDraft(null);
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
        {error && secrets.length === 0 ? (
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
        ) : secrets.length === 0 ? (
          <p className="text-muted-foreground flex h-9 items-center text-sm">
            {t("agentEnvironment.secrets.empty")}
          </p>
        ) : (
          <ul
            className="divide-y rounded-md border"
            data-testid="agent-secret-names"
          >
            {secrets.map((secret) => (
              <li
                key={secret.name}
                className="flex h-9 items-center gap-2 pr-1 pl-3"
              >
                {/* The name keeps its width up to 60% of the row; the scope
                    badge truncates first. */}
                <span className="max-w-[60%] shrink-0 truncate font-mono text-sm">
                  {secret.name}
                </span>
                <SecretScopeBadge secret={secret} />
                {canManage ? (
                  <div className="ml-auto flex shrink-0 items-center">
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="size-7"
                      aria-label={t("agentEnvironment.secrets.editLabel", {
                        name: secret.name,
                      })}
                      title={t("agentEnvironment.secrets.editLabel", {
                        name: secret.name,
                      })}
                      disabled={saving}
                      onClick={() => edit(secret)}
                    >
                      <Pencil className="size-4" />
                    </Button>
                    <ConfirmDialog
                      open={deleting === secret.name}
                      onOpenChange={(open) =>
                        setDeleting(open ? secret.name : null)
                      }
                      trigger={
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          className="size-7"
                          aria-label={t(
                            "agentEnvironment.secrets.deleteLabel",
                            { name: secret.name },
                          )}
                          disabled={saving}
                        >
                          <Trash2 className="size-4" />
                        </Button>
                      }
                      title={t("agentEnvironment.secrets.deleteTitle", {
                        name: secret.name,
                      })}
                      description={t("agentEnvironment.secrets.deleteBody")}
                      confirmLabel={t("agentEnvironment.secrets.deleteConfirm")}
                      destructive
                      closeOnConfirm={false}
                      pending={saving && deleting === secret.name}
                      onConfirm={() => void confirmDelete(secret.name)}
                    />
                  </div>
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
              placeholder={
                existing
                  ? t("agentEnvironment.secrets.valueKeepPlaceholder")
                  : t("agentEnvironment.secrets.valuePlaceholder")
              }
              aria-label={t("agentEnvironment.secrets.valueLabel")}
              aria-invalid={valueProblem != null}
              disabled={!canManage}
              {...SECRET_INPUT_PROPS}
              className="min-w-48 flex-1 font-mono"
            />
          </div>
          <div className="flex max-w-2xl flex-wrap gap-2">
            <Select
              value={scope}
              onValueChange={(next) =>
                changeScope({ scope: next as AgentSecretScope, repo })
              }
              disabled={!canManage}
            >
              <SelectTrigger
                ref={scopeTrigger}
                className="w-44"
                aria-label={t("agentEnvironment.secrets.scopeLabel")}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {AGENT_SECRET_SCOPES.map((option) => (
                  <SelectItem key={option} value={option}>
                    {t(SCOPE_OPTION_KEYS[option])}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {scope === "repo" ? (
              <SecretRepoField
                value={repo}
                onChange={(next) => changeScope({ scope, repo: next })}
                disabled={!canManage}
              />
            ) : null}
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

/** Which sessions receive the secret, as a list row shows it. */
function SecretScopeBadge({ secret }: { secret: AgentSecretView }) {
  const { t } = useTranslations();
  const label =
    secret.scope === "repo"
      ? t("agentEnvironment.secrets.scopeBadgeRepo", {
          repo: secret.repo ?? "",
        })
      : t(SCOPE_OPTION_KEYS[secret.scope]);
  return (
    <Badge
      variant="secondary"
      className="min-w-0 shrink font-normal"
      title={label}
      data-testid="agent-secret-scope"
    >
      <span className="truncate">{label}</span>
    </Badge>
  );
}

/**
 * The repo scope's repository: the workspace's connected repositories to pick
 * from, or any owner/name typed in. Mounted only for the repo scope, so the
 * repository list loads only when it is needed.
 */
function SecretRepoField({
  value,
  onChange,
  disabled,
}: {
  value: string;
  onChange: (value: string) => void;
  disabled: boolean;
}) {
  const { t } = useTranslations();
  return (
    <ConnectedRepoCombobox
      value={value}
      onChange={onChange}
      placeholder={t("agentEnvironment.secrets.repoPlaceholder")}
      emptyText={t("agentEnvironment.secrets.repoEmpty")}
      ariaLabel={t("agentEnvironment.secrets.repoLabel")}
      disabled={disabled}
      className="w-56 font-mono"
    />
  );
}
