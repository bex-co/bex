import { useMemo, useState } from "react";
import { Github, GitBranch, Box } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/common/components/ui/tabs";
import { isValidGitUrl } from "@/common/lib/utils/git-url";
import { cn } from "@/common/lib/utils/utils";
import { useRepos, type RepoView } from "@/features/services/hooks/use-repos";
import { useGitConnection } from "@/features/git/hooks/use-git-connection";
import { RegistryCredentialSelect } from "@/features/services/components/registry-credential-select";

/** Where a new service's code comes from. */
export type SourceTab = "github" | "git" | "image";

const TAB_TRIGGER_CLASS =
  "min-w-0 px-1 text-xs sm:gap-1.5 sm:px-3 sm:text-sm" as const;

/**
 * Source selection for the create-service form: a connected GitHub repo, a
 * public git URL, or a prebuilt image.
 *
 * The repo search box, the repo list, and the GitHub connection state live
 * here because nothing outside this picker reads them. The selection itself
 * stays lifted — the create payload and the name auto-fill both depend on it.
 */
export function ServiceSourcePicker({
  tab,
  onTabChange,
  selectedRepo,
  onSelectRepo,
  gitUrl,
  onGitUrlChange,
  image,
  onImageChange,
  registryCredentialId,
  onRegistryCredentialChange,
}: {
  tab: SourceTab;
  onTabChange: (tab: SourceTab) => void;
  selectedRepo: RepoView | null;
  onSelectRepo: (repo: RepoView) => void;
  gitUrl: string;
  onGitUrlChange: (url: string) => void;
  image: string;
  onImageChange: (image: string) => void;
  registryCredentialId: string;
  onRegistryCredentialChange: (id: string) => void;
}) {
  const { t } = useTranslations();
  const { repos, loading: reposLoading } = useRepos();
  const { connection, loading: connectionLoading } = useGitConnection();
  const [repoSearch, setRepoSearch] = useState("");

  const gitHubDisconnected =
    !connectionLoading && connection?.connected !== true;

  const filteredRepos = useMemo(
    () =>
      repos.filter(
        (r) =>
          !repoSearch ||
          r.fullName.toLowerCase().includes(repoSearch.toLowerCase()),
      ),
    [repos, repoSearch],
  );

  return (
    <div className="space-y-3">
      <Label>{t("services.createSourceTitle")}</Label>
      <Tabs value={tab} onValueChange={(v) => onTabChange(v as SourceTab)}>
        <TabsList className="grid h-auto w-full grid-cols-3">
          <TabsTrigger value="github" className={TAB_TRIGGER_CLASS}>
            <Github className="hidden size-4 sm:block" />
            {t("services.createTabGitHub")}
          </TabsTrigger>
          <TabsTrigger value="git" className={TAB_TRIGGER_CLASS}>
            <GitBranch className="hidden size-4 sm:block" />
            {t("services.createTabPublicGit")}
          </TabsTrigger>
          <TabsTrigger value="image" className={TAB_TRIGGER_CLASS}>
            <Box className="hidden size-4 sm:block" />
            {t("services.createTabImage")}
          </TabsTrigger>
        </TabsList>

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
                placeholder={t("services.createRepoSearchPlaceholder")}
                value={repoSearch}
                onChange={(e) => setRepoSearch(e.target.value)}
                aria-label={t("services.createRepoSearchPlaceholder")}
              />
              <div className="max-h-64 overflow-y-auto rounded-md border divide-y">
                {reposLoading ? (
                  Array.from({ length: 3 }).map((_, i) => (
                    <div key={i} className="flex items-center gap-3 p-3">
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
                      onClick={() => onSelectRepo(r)}
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

        <TabsContent value="git" className="mt-3">
          <div className="space-y-2">
            <Label htmlFor="svc-git-url">
              {t("services.createPublicUrlLabel")}
            </Label>
            <Input
              id="svc-git-url"
              value={gitUrl}
              onChange={(e) => onGitUrlChange(e.target.value)}
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

        <TabsContent value="image" className="mt-3">
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="svc-image">
                {t("services.createImageLabel")}
              </Label>
              <Input
                id="svc-image"
                value={image}
                onChange={(e) => onImageChange(e.target.value)}
                placeholder={t("services.createImagePlaceholder")}
                autoComplete="off"
              />
              <p className="text-xs text-muted-foreground">
                {t("services.createImagePortHint")}
              </p>
            </div>
            <RegistryCredentialSelect
              id="svc-registry-credential-image"
              value={registryCredentialId}
              onValueChange={onRegistryCredentialChange}
              description={t("services.createRegistryCredentialDescription")}
            />
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}
