import { useEffect, useMemo, useState } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Github, GitBranch, Loader2, RefreshCw } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Badge } from "@/common/components/ui/badge";
import { Skeleton } from "@/common/components/ui/skeleton";
import { Combobox } from "@/common/components/ui/combobox";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/common/components/ui/tabs";
import { cn } from "@/common/lib/utils/utils";
import { repoNameSlug, gitUrlSlug } from "@/common/lib/utils/slug";
import { isValidGitUrl } from "@/common/lib/utils/git-url";
import { useRepos } from "@/features/services/hooks/use-repos";
import type { RepoView } from "@/features/services/hooks/use-repos";
import { useRepoBranches } from "@/features/services/hooks/use-repo-branches";
import { useGitConnection } from "@/features/git/hooks/use-git-connection";
import { useCreateBlueprint } from "@/features/blueprints/hooks/use-create-blueprint";
import { useBlueprintPreview } from "@/features/blueprints/hooks/use-blueprint-preview";
import { EstimatedPricingPanel } from "@/features/blueprints/components/estimated-pricing-panel";
import { ProtectedConfirmationDialog } from "@/common/components/protected-confirmation-dialog";
import { protectedServiceName } from "@/features/services/lib/protected-confirmation";

export const Route = createFileRoute("/blueprints/new")({
  staticData: { chrome: true },
  component: NewBlueprintPage,
  beforeLoad: requireAuth(),
  head: ({ match }) => translatedTitleHead("blueprints.createTitle", match),
});

type SourceTab = "github" | "git";

/** One named group of planned resources in the pre-create review. */
function PlanGroup({ label, names }: { label: string; names: string[] }) {
  if (names.length === 0) return null;
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <span className="text-sm text-muted-foreground">{label}:</span>
      {names.map((n) => (
        <Badge key={n} variant="secondary" className="text-xs">
          {n}
        </Badge>
      ))}
    </div>
  );
}

export function NewBlueprintPage() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { repos, loading: reposLoading } = useRepos();
  const { connection, loading: connectionLoading } = useGitConnection();
  const { create, busy } = useCreateBlueprint();

  const [tab, setTab] = useState<SourceTab>("github");
  const [selectedRepo, setSelectedRepo] = useState<RepoView | null>(null);
  const [repoSearch, setRepoSearch] = useState("");
  const [gitUrl, setGitUrl] = useState("");
  const [name, setName] = useState("");
  const [nameEdited, setNameEdited] = useState(false);
  const [branch, setBranch] = useState("");
  const [path, setPath] = useState("render.yaml");
  const [protectedConfirmation, setProtectedConfirmation] = useState<
    string | null
  >(null);

  const sourceRepo =
    tab === "github"
      ? (selectedRepo?.cloneUrl ?? "")
      : isValidGitUrl(gitUrl)
        ? gitUrl.trim()
        : "";

  const { branches } = useRepoBranches(sourceRepo || null);
  const {
    preview,
    loading: previewLoading,
    error: previewError,
    refetch: refetchPreview,
  } = useBlueprintPreview(sourceRepo, branch.trim(), path.trim());

  // Auto-fill name + branch from the selected source while the user hasn't
  // hand-edited the name (same pattern as services.new: the effect reacts to
  // selection changes, so a synchronous set is the desired behavior).
  useEffect(() => {
    if (tab === "github" && selectedRepo) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setBranch((b) => b || selectedRepo.defaultBranch);
      if (!nameEdited) {
        setName(repoNameSlug(selectedRepo.fullName));
      }
    } else if (tab === "git" && gitUrl && !nameEdited) {
      const slug = gitUrlSlug(gitUrl);
      if (slug) setName(slug);
    }
  }, [tab, selectedRepo, gitUrl, nameEdited]);

  const filteredRepos = useMemo(
    () =>
      repos.filter(
        (r) =>
          !repoSearch ||
          r.fullName.toLowerCase().includes(repoSearch.toLowerCase()),
      ),
    [repos, repoSearch],
  );

  const branchOptions = useMemo(
    () => branches.map((b) => ({ value: b, label: b })),
    [branches],
  );

  const sourceValid = sourceRepo !== "";
  // Render parity: Deploy stays disabled until the file fetches + parses. A
  // transport-level preview failure does not block — the backend re-validates
  // on create anyway.
  const previewBlocks =
    preview != null && (!preview.found || preview.validation?.valid !== true);
  const canSubmit =
    sourceValid &&
    branch.trim() !== "" &&
    !busy &&
    !previewLoading &&
    !previewBlocks;

  async function handleCreate(confirmation?: string) {
    const result = await create(
      sourceRepo,
      branch.trim(),
      path.trim() || "render.yaml",
      name.trim(),
      confirmation,
    );
    if (result.status === "confirmation_required") {
      setProtectedConfirmation(result.confirmation);
      return;
    }
    if (result.status === "success") {
      setProtectedConfirmation(null);
      void navigate({
        to: "/blueprints/$blueprintId",
        params: { blueprintId: result.blueprint.id },
      });
    }
  }

  const gitHubDisconnected =
    !connectionLoading && connection?.connected !== true;

  const plan = preview?.validation?.plan;
  const validationErrors = (preview?.validation?.errors ?? []).filter(
    (e): e is string => !!e,
  );

  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-2xl space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>
                <h1 className="text-xl font-semibold">
                  {t("blueprints.createTitle")}
                </h1>
              </CardTitle>
              <CardDescription>
                {t("blueprints.createDescription")}
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              {/* Source picker — same GitHub-integration pattern as services.new */}
              <div className="space-y-3">
                <Label>{t("blueprints.createSourceTitle")}</Label>
                <Tabs
                  value={tab}
                  onValueChange={(v) => {
                    setTab(v as SourceTab);
                    setSelectedRepo(null);
                    setBranch("");
                  }}
                >
                  <TabsList className="grid h-auto w-full grid-cols-2">
                    <TabsTrigger
                      value="github"
                      className="min-w-0 px-1 text-xs sm:gap-1.5 sm:px-3 sm:text-sm"
                    >
                      <Github className="hidden size-4 sm:block" />
                      {t("services.createTabGitHub")}
                    </TabsTrigger>
                    <TabsTrigger
                      value="git"
                      className="min-w-0 px-1 text-xs sm:gap-1.5 sm:px-3 sm:text-sm"
                    >
                      <GitBranch className="hidden size-4 sm:block" />
                      {t("services.createTabPublicGit")}
                    </TabsTrigger>
                  </TabsList>

                  {/* GitHub tab */}
                  <TabsContent value="github" className="mt-3">
                    {connectionLoading ? (
                      <Skeleton className="h-24 w-full" />
                    ) : gitHubDisconnected ? (
                      <div className="flex flex-col items-center gap-3 rounded-lg border p-6 text-center">
                        <Github className="size-8 text-muted-foreground" />
                        <div>
                          <p className="font-medium">
                            {t("services.createGitConnectPromptTitle")}
                          </p>
                          <p className="mt-1 text-sm text-muted-foreground">
                            {t("services.createGitConnectPromptBody")}
                          </p>
                        </div>
                        <Button asChild>
                          <a
                            href={connection?.installUrl ?? ""}
                            target="_blank"
                            rel="noreferrer"
                          >
                            {t("services.createGitConnectButton")}
                          </a>
                        </Button>
                      </div>
                    ) : (
                      <div className="space-y-2">
                        <Input
                          placeholder={t(
                            "services.createRepoSearchPlaceholder",
                          )}
                          value={repoSearch}
                          onChange={(e) => setRepoSearch(e.target.value)}
                          aria-label={t("services.createRepoSearchPlaceholder")}
                        />
                        <div className="max-h-64 overflow-y-auto rounded-md border divide-y">
                          {reposLoading ? (
                            Array.from({ length: 3 }).map((_, i) => (
                              <div
                                key={i}
                                className="flex items-center gap-3 p-3"
                              >
                                <Skeleton className="h-4 w-full" />
                              </div>
                            ))
                          ) : filteredRepos.length === 0 ? (
                            <div className="p-6 text-center text-sm text-muted-foreground">
                              {repoSearch
                                ? t("services.createRepoNoMatch")
                                : t("services.createRepoEmpty")}
                            </div>
                          ) : (
                            filteredRepos.map((r) => (
                              <button
                                key={r.id}
                                type="button"
                                onClick={() => setSelectedRepo(r)}
                                className={cn(
                                  "flex w-full items-center justify-between p-3 text-left transition-colors hover:bg-muted",
                                  selectedRepo?.id === r.id &&
                                    "bg-primary/5 hover:bg-primary/10",
                                )}
                              >
                                <div className="flex min-w-0 items-center gap-2">
                                  <Github className="size-4 shrink-0 text-muted-foreground" />
                                  <span className="truncate text-sm font-medium">
                                    {r.fullName}
                                  </span>
                                  {r.private && (
                                    <Badge
                                      variant="secondary"
                                      className="shrink-0 text-xs"
                                    >
                                      {t("services.createRepoPrivateBadge")}
                                    </Badge>
                                  )}
                                </div>
                                <span className="ml-3 shrink-0 text-xs text-muted-foreground">
                                  {r.defaultBranch}
                                </span>
                              </button>
                            ))
                          )}
                        </div>
                      </div>
                    )}
                  </TabsContent>

                  {/* Public Git URL tab */}
                  <TabsContent value="git" className="mt-3">
                    <div className="space-y-2">
                      <Label htmlFor="bp-git-url">
                        {t("services.createPublicUrlLabel")}
                      </Label>
                      <Input
                        id="bp-git-url"
                        value={gitUrl}
                        onChange={(e) => setGitUrl(e.target.value)}
                        placeholder={t("services.createPublicUrlPlaceholder")}
                        autoComplete="off"
                      />
                      {gitUrl && !isValidGitUrl(gitUrl) ? (
                        <p className="text-sm text-destructive">
                          {t("services.createPublicUrlError")}
                        </p>
                      ) : null}
                    </div>
                  </TabsContent>
                </Tabs>
              </div>

              {/* Settings */}
              <div className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="bp-name">
                    {t("blueprints.createNameLabel")}
                  </Label>
                  <Input
                    id="bp-name"
                    value={name}
                    onChange={(e) => {
                      setName(e.target.value);
                      setNameEdited(e.target.value !== "");
                    }}
                    placeholder={t("blueprints.createNamePlaceholder")}
                    autoComplete="off"
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="bp-branch">
                    {t("blueprints.createBranchLabel")}
                  </Label>
                  <Combobox
                    options={branchOptions}
                    value={branch}
                    onValueChange={setBranch}
                    placeholder={t("blueprints.createBranchPlaceholder")}
                    ariaLabel={t("blueprints.createBranchLabel")}
                    allowCustom
                  />
                  <p className="text-sm text-muted-foreground">
                    {t("blueprints.createBranchHint")}
                  </p>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="bp-path">
                    {t("blueprints.createPathLabel")}
                  </Label>
                  <Input
                    id="bp-path"
                    value={path}
                    onChange={(e) => setPath(e.target.value)}
                    placeholder={t("blueprints.createPathPlaceholder")}
                    autoComplete="off"
                  />
                  <p className="text-sm text-muted-foreground">
                    {t("blueprints.createPathHint")}
                  </p>
                </div>
              </div>

              {/* Pre-create review (Render's "Review Blueprint configurations") */}
              <div className="space-y-3">
                <p className="text-base font-semibold">
                  {t("blueprints.previewTitle")}
                </p>
                {!sourceValid || !branch.trim() ? (
                  <p className="text-sm text-muted-foreground">
                    {t("blueprints.previewSelectSource")}
                  </p>
                ) : previewLoading ? (
                  <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    <Loader2 className="size-4 animate-spin" />
                    {t("blueprints.previewLoading", {
                      path: path || "render.yaml",
                    })}
                  </div>
                ) : preview && !preview.found ? (
                  <Alert variant="destructive">
                    <AlertTitle>
                      {t("blueprints.previewNotFoundTitle")}
                    </AlertTitle>
                    <AlertDescription className="flex flex-col gap-2">
                      <span>
                        {preview.error ||
                          t("blueprints.previewNotFoundBody", {
                            path: path || "render.yaml",
                            branch,
                          })}
                      </span>
                      <Button
                        variant="outline"
                        size="sm"
                        className="self-start"
                        onClick={() => void refetchPreview()}
                      >
                        <RefreshCw className="size-3.5" />
                        {t("blueprints.previewRetry")}
                      </Button>
                    </AlertDescription>
                  </Alert>
                ) : preview && preview.validation?.valid !== true ? (
                  <Alert variant="destructive">
                    <AlertTitle>{t("blueprints.previewInvalid")}</AlertTitle>
                    <AlertDescription>
                      <ul className="list-disc space-y-1 pl-4">
                        {validationErrors.map((e, i) => (
                          <li key={i}>{e}</li>
                        ))}
                      </ul>
                    </AlertDescription>
                  </Alert>
                ) : preview ? (
                  <>
                    <div className="space-y-3 rounded-md border p-4">
                      <p className="text-sm font-medium">
                        {t("blueprints.previewValid", {
                          count: plan?.totalActions ?? 0,
                        })}
                      </p>
                      <PlanGroup
                        label={t("blueprints.previewServices")}
                        names={(plan?.services ?? []).filter(Boolean)}
                      />
                      <PlanGroup
                        label={t("blueprints.previewDatabases")}
                        names={(plan?.databases ?? []).filter(Boolean)}
                      />
                      <PlanGroup
                        label={t("blueprints.previewKeyValue")}
                        names={(plan?.keyValue ?? []).filter(Boolean)}
                      />
                      <PlanGroup
                        label={t("blueprints.previewEnvGroups")}
                        names={(plan?.envGroups ?? []).filter(Boolean)}
                      />
                      <p className="text-sm text-muted-foreground">
                        {t("blueprints.previewAutoSyncNote")}
                      </p>
                    </div>
                    <EstimatedPricingPanel
                      pricing={preview.validation?.estimatedPricing}
                    />
                  </>
                ) : previewError ? (
                  <Alert variant="destructive">
                    <AlertDescription className="flex flex-col gap-2">
                      <span>{t("blueprints.previewError")}</span>
                      <Button
                        variant="outline"
                        size="sm"
                        className="self-start"
                        onClick={() => void refetchPreview()}
                      >
                        <RefreshCw className="size-3.5" />
                        {t("blueprints.previewRetry")}
                      </Button>
                    </AlertDescription>
                  </Alert>
                ) : null}
              </div>

              <div className="flex justify-end gap-2">
                <Button
                  variant="outline"
                  onClick={() => void navigate({ to: "/blueprints" })}
                  disabled={busy}
                >
                  {t("blueprints.createCancel")}
                </Button>
                <Button
                  onClick={() => void handleCreate()}
                  disabled={!canSubmit}
                >
                  {busy ? <Loader2 className="animate-spin" /> : null}
                  {t("blueprints.createAction")}
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>

      <ProtectedConfirmationDialog
        key={protectedConfirmation ? `open:${protectedConfirmation}` : "closed"}
        open={protectedConfirmation !== null}
        resourceName={
          protectedConfirmation
            ? protectedServiceName(protectedConfirmation)
            : name || sourceRepo
        }
        requiredConfirmation={protectedConfirmation ?? ""}
        actionLabel={t("blueprints.createAction")}
        busy={busy}
        onOpenChange={(open) => !open && setProtectedConfirmation(null)}
        onConfirm={async (confirmation) => {
          await handleCreate(confirmation);
        }}
      />
    </DashboardLayout>
  );
}
