import { useState } from "react";
import { Eye, Pencil, Plus, Trash2 } from "lucide-react";
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
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { hasGraphQLErrorCode } from "@/common/lib/graphql-error";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useAgentRepoEnvironments } from "@/features/agent-environment/hooks/use-agent-repo-environments";
import { AgentRepoEnvironmentsCardSkeleton } from "@/features/agent-environment/components/agent-repo-environments-card-skeleton";
import {
  AgentRepoEnvironmentDialog,
  type RepoEnvironmentDialogTarget,
} from "@/features/agent-environment/components/agent-repo-environment-dialog";
import {
  REPO_ENVIRONMENTS_ANCHOR,
  repoEnvironmentErrorKey,
  type RepoEnvironmentView,
  type RepoStep,
  type RepoEnvironmentInput,
} from "@/features/agent-environment/lib/repo-environments";

/**
 * Settings → Repository environments (ADR109): per repository, the setup
 * steps a session runs from the repository root before its agent starts, and
 * the checks the agent runs before it delivers. Members who can start
 * sessions see the list and can open an environment; only admins add, edit or
 * delete one. Each save replaces the repository's whole environment.
 */
export function AgentRepoEnvironmentsCard({ ownerId }: { ownerId: string }) {
  const { t } = useTranslations();
  const { canManage } = useCapabilities();
  const { environments, loading, error, refetch, set, remove, saving } =
    useAgentRepoEnvironments(ownerId);
  const [dialog, setDialog] = useState<RepoEnvironmentDialogTarget | null>(
    null,
  );
  const [errorKey, setErrorKey] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);

  if (loading) return <AgentRepoEnvironmentsCardSkeleton />;

  async function save(repo: string, environment: RepoEnvironmentInput) {
    await set(repo, environment);
    toast.success(t("agentEnvironment.repoEnvs.saved", { repo }));
    setDialog(null);
  }

  async function confirmDelete(repo: string) {
    setErrorKey(null);
    try {
      await remove(repo);
      toast.success(t("agentEnvironment.repoEnvs.deleted", { repo }));
    } catch (err) {
      setErrorKey(repoEnvironmentErrorKey(err));
    } finally {
      setDeleting(null);
    }
  }

  return (
    <Card
      data-testid="agent-repo-environments-card"
      id={REPO_ENVIRONMENTS_ANCHOR}
    >
      <CardHeader>
        <CardTitle>{t("agentEnvironment.repoEnvs.title")}</CardTitle>
        <CardDescription>
          {t("agentEnvironment.repoEnvs.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {error && environments.length === 0 ? (
          <Alert variant="destructive">
            <AlertDescription className="flex flex-wrap items-center gap-3">
              <span>
                {hasGraphQLErrorCode(
                  error,
                  "AGENT_REPO_ENVIRONMENTS_UNAVAILABLE",
                )
                  ? t("agentEnvironment.repoEnvs.unavailable")
                  : t("agentEnvironment.repoEnvs.loadFailed")}
              </span>
              <Button
                size="sm"
                variant="outline"
                onClick={() => void refetch()}
              >
                {t("agentEnvironment.retry")}
              </Button>
            </AlertDescription>
          </Alert>
        ) : environments.length === 0 ? (
          // The same box as one row inside the list's border, so the
          // skeleton matches either.
          <p className="text-muted-foreground box-content flex h-14 items-center border border-transparent text-sm">
            {t("agentEnvironment.repoEnvs.emptyList")}
          </p>
        ) : (
          <ul
            className="divide-y rounded-md border"
            data-testid="agent-repo-environments"
          >
            {environments.map((env) => (
              <li
                key={env.repo}
                className="flex h-14 items-center gap-3 pr-1 pl-3"
              >
                <div className="min-w-0 flex-1">
                  <p className="truncate font-mono text-sm">{env.repo}</p>
                  <RepoEnvironmentSummary environment={env} />
                </div>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="size-7"
                  aria-label={t(
                    canManage
                      ? "agentEnvironment.repoEnvs.editLabel"
                      : "agentEnvironment.repoEnvs.viewLabel",
                    { repo: env.repo },
                  )}
                  title={t(
                    canManage
                      ? "agentEnvironment.repoEnvs.editLabel"
                      : "agentEnvironment.repoEnvs.viewLabel",
                    { repo: env.repo },
                  )}
                  disabled={saving}
                  onClick={() => {
                    setErrorKey(null);
                    setDialog({
                      kind: canManage ? "edit" : "view",
                      environment: env,
                    });
                  }}
                >
                  {canManage ? (
                    <Pencil className="size-4" />
                  ) : (
                    <Eye className="size-4" />
                  )}
                </Button>
                {canManage ? (
                  <ConfirmDialog
                    open={deleting === env.repo}
                    onOpenChange={(open) => setDeleting(open ? env.repo : null)}
                    trigger={
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="size-7"
                        aria-label={t("agentEnvironment.repoEnvs.deleteLabel", {
                          repo: env.repo,
                        })}
                        disabled={saving}
                      >
                        <Trash2 className="size-4" />
                      </Button>
                    }
                    title={t("agentEnvironment.repoEnvs.deleteTitle", {
                      repo: env.repo,
                    })}
                    description={t("agentEnvironment.repoEnvs.deleteBody")}
                    confirmLabel={t("agentEnvironment.repoEnvs.deleteConfirm")}
                    destructive
                    closeOnConfirm={false}
                    pending={saving && deleting === env.repo}
                    onConfirm={() => void confirmDelete(env.repo)}
                  />
                ) : null}
              </li>
            ))}
          </ul>
        )}
        {/* Rendered for everyone, disabled for non-admins, so the loaded card
            keeps its skeleton's geometry. */}
        <div className="space-y-2 border-t pt-4">
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="outline"
              disabled={!canManage}
              onClick={() => {
                setErrorKey(null);
                setDialog({ kind: "create" });
              }}
            >
              <Plus />
              {t("agentEnvironment.repoEnvs.add")}
            </Button>
          </div>
          <p className="text-muted-foreground text-xs">
            {canManage
              ? t("agentEnvironment.repoEnvs.help")
              : t("agentEnvironment.repoEnvs.readOnly")}
          </p>
          {errorKey ? (
            <p className="text-destructive text-sm" role="alert">
              {t(errorKey)}
            </p>
          ) : null}
        </div>
      </CardContent>
      {dialog ? (
        <AgentRepoEnvironmentDialog
          key={dialog.kind === "create" ? "create" : dialog.environment.repo}
          target={dialog}
          existingRepos={environments.map((env) => env.repo)}
          onOpenChange={(open) => {
            if (!open) setDialog(null);
          }}
          onSave={save}
          saving={saving}
        />
      ) : null}
    </Card>
  );
}

/** A row's second line: its setup steps, then its checks, by name. */
function RepoEnvironmentSummary({
  environment,
}: {
  environment: RepoEnvironmentView;
}) {
  const { t } = useTranslations();
  const names = (steps: RepoStep[]) => steps.map((s) => s.name).join(", ");
  const setup =
    environment.setup.length > 0
      ? t("agentEnvironment.repoEnvs.setupSummary", {
          names: names(environment.setup),
        })
      : t("agentEnvironment.repoEnvs.noSetup");
  const checks =
    environment.checks.length > 0
      ? t("agentEnvironment.repoEnvs.checksSummary", {
          names: names(environment.checks),
        })
      : t("agentEnvironment.repoEnvs.noChecks");
  const summary = `${setup} · ${checks}`;
  return (
    <p
      className="text-muted-foreground truncate text-xs"
      title={summary}
      data-testid="agent-repo-environment-summary"
    >
      {summary}
    </p>
  );
}
