import { useMemo, useState } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Combobox } from "@/common/components/ui/combobox";
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
import { isValidGitUrl } from "@/common/lib/utils/git-url";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  ServiceSourcePicker,
  type SourceTab,
} from "@/features/services/components/service-source-picker";
import { useRepoBranches } from "@/features/services/hooks/use-repo-branches";
import type { RepoView } from "@/features/services/hooks/use-repos";
import { useSetImage } from "@/features/services/hooks/use-set-image";
import { useSetRepo } from "@/features/services/hooks/use-set-repo";
import { formatRepoLabel } from "@/features/services/lib/repo";

export interface ServiceSourceCardProps {
  serviceId: string;
  repo: string | null;
  branch: string | null;
  imagePath: string | null;
  registryCredentialId: string | null;
}

/**
 * Render-style Settings → Source card. The ready state has one stable card for
 * both source kinds; Edit opens the shared account-grouped picker and lets the
 * user repoint repo+branch atomically or switch to a prebuilt image. Updating
 * source intent never deploys it — the next explicit/push deploy consumes it.
 */
export function ServiceSourceCard(props: ServiceSourceCardProps) {
  const { t } = useTranslations();
  const { canCreate } = useCapabilities();
  const reason = canCreate ? undefined : t("capabilities.reasonCanCreate");

  return (
    <Card>
      <CardHeader className="flex-row items-start justify-between gap-4">
        <div className="space-y-1.5">
          <CardTitle>{t("services.sourceTitle")}</CardTitle>
          <CardDescription>
            {t(
              props.repo
                ? "services.sourceRepoDescription"
                : "services.sourceImageDescription",
            )}
          </CardDescription>
        </div>
        <SourceEditDialog {...props} disabled={!canCreate} />
      </CardHeader>
      <CardContent>
        <dl className="grid gap-4 text-sm sm:grid-cols-2">
          <div className="space-y-1">
            <dt className="text-muted-foreground">
              {t(
                props.repo
                  ? "services.buildDeploySourceLabel"
                  : "services.sourceImageLabel",
              )}
            </dt>
            <dd className="break-all font-mono font-medium">
              {props.repo
                ? formatRepoLabel(props.repo)
                : props.imagePath || t("services.sourceMissing")}
            </dd>
          </div>
          {props.repo && (
            <div className="space-y-1">
              <dt className="text-muted-foreground">
                {t("services.buildDeployBranchLabel")}
              </dt>
              <dd className="break-all font-mono font-medium">
                {props.branch || "main"}
              </dd>
            </div>
          )}
        </dl>
        {reason && (
          <p className="mt-4 text-sm text-muted-foreground">{reason}</p>
        )}
      </CardContent>
    </Card>
  );
}

function SourceEditDialog({
  serviceId,
  repo,
  branch: currentBranch,
  imagePath,
  registryCredentialId,
  disabled,
}: ServiceSourceCardProps & { disabled: boolean }) {
  const { t } = useTranslations();
  const { setRepo, busy: repoBusy } = useSetRepo();
  const { setImage, busy: imageBusy } = useSetImage();
  const [open, setOpen] = useState(false);
  const [tab, setTab] = useState<SourceTab>(repo ? "git" : "image");
  const [selectedRepo, setSelectedRepo] = useState<RepoView | null>(null);
  const [gitUrl, setGitUrl] = useState(repo ?? "");
  const [branch, setBranch] = useState(currentBranch ?? "main");
  const [image, setImageValue] = useState(imagePath ?? "");
  const [credentialId, setCredentialId] = useState(registryCredentialId ?? "");

  const sourceRepo = tab === "github" ? (selectedRepo?.htmlUrl ?? "") : gitUrl;
  const { branches } = useRepoBranches(
    open && tab !== "image" ? sourceRepo || null : null,
  );
  const branchOptions = useMemo(
    () => branches.map((value) => ({ value, label: value })),
    [branches],
  );
  const valid =
    tab === "image"
      ? image.trim() !== ""
      : tab === "github"
        ? selectedRepo !== null
        : isValidGitUrl(gitUrl);
  const dirty =
    tab === "image"
      ? repo !== null ||
        image.trim() !== (imagePath ?? "").trim() ||
        (credentialId.trim() || null) !== (registryCredentialId?.trim() || null)
      : repo === null ||
        sourceRepo.trim() !== repo.trim() ||
        (branch.trim() || "main") !== (currentBranch?.trim() || "main");
  const busy = repoBusy || imageBusy;

  function resetDraft() {
    setTab(repo ? "git" : "image");
    setSelectedRepo(null);
    setGitUrl(repo ?? "");
    setBranch(currentBranch || "main");
    setImageValue(imagePath ?? "");
    setCredentialId(registryCredentialId ?? "");
  }

  function changeOpen(next: boolean) {
    if (next) resetDraft();
    setOpen(next);
  }

  function selectRepo(next: RepoView) {
    setSelectedRepo(next);
    setBranch(next.defaultBranch || "main");
  }

  function changeTab(next: SourceTab) {
    setTab(next);
    if (next === "github" && selectedRepo && !branch) {
      setBranch(selectedRepo.defaultBranch || "main");
    }
  }

  async function save() {
    const ok =
      tab === "image"
        ? await setImage(serviceId, image.trim(), credentialId || undefined)
        : await setRepo(serviceId, {
            repo: sourceRepo.trim(),
            branch: branch.trim() || "main",
          });
    if (!ok) return;
    setOpen(false);
  }

  return (
    <Dialog open={open} onOpenChange={changeOpen}>
      <Button
        variant="outline"
        size="sm"
        disabled={disabled}
        onClick={() => changeOpen(true)}
      >
        {t("services.sourceEdit")}
      </Button>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{t("services.sourceDialogTitle")}</DialogTitle>
          <DialogDescription>
            {t("services.sourceNoAutoDeploy")}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <ServiceSourcePicker
            tab={tab}
            onTabChange={changeTab}
            selectedRepo={selectedRepo}
            onSelectRepo={selectRepo}
            gitUrl={gitUrl}
            onGitUrlChange={setGitUrl}
            image={{
              value: image,
              onChange: setImageValue,
              registryCredentialId: credentialId,
              onRegistryCredentialChange: setCredentialId,
              showPortHint: false,
            }}
            titleKey="services.sourcePickerLabel"
            idPrefix="update-source"
          />
          {tab !== "image" && valid && (
            <div className="space-y-2">
              <Label>{t("services.buildDeployBranchLabel")}</Label>
              <Combobox
                value={branch}
                onValueChange={setBranch}
                options={branchOptions}
                allowCustom
                placeholder={t("services.buildDeployBranchPlaceholder")}
                emptyText={t("services.sourceBranchEmpty")}
                ariaLabel={t("services.buildDeployBranchLabel")}
                className="font-mono"
              />
              <p className="text-xs text-muted-foreground">
                {t("services.sourceBranchHint")}
              </p>
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={() => setOpen(false)}>
            {t("services.buildDeployCancel")}
          </Button>
          <Button
            disabled={!valid || !dirty || busy}
            onClick={() => void save()}
          >
            {busy && <Loader2 className="animate-spin" />}
            {t("services.sourceUpdateSave")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
