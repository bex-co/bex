import { useState } from "react";
import { Loader2 } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";
import { RegistryCredentialSelect } from "@/features/services/components/registry-credential-select";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useSetImage } from "@/features/services/hooks/use-set-image";
import { useSetRepo } from "@/features/services/hooks/use-set-repo";
import { useRepos } from "@/features/services/hooks/use-repos";
import { isValidGitUrl } from "@/common/lib/utils/git-url";

// A source-kind switch chooses the executable the operator runs (can_create,
// docs/ADR024-members.md); the shared reason string mirrors BuildDeploySection.
function useSourceKindGate() {
  const { t } = useTranslations();
  const { canCreate } = useCapabilities();
  return {
    disabled: !canCreate,
    reason: !canCreate ? t("capabilities.reasonCanCreate") : undefined,
  };
}

/**
 * ImageSourceCard is the Settings Source card for an **image-backed** service
 * (w5/m76, ADR026 §8). Before this, an image-backed service had no source UI at
 * all — its configured image was invisible and unchangeable from the dashboard.
 * The card shows the image (editable inline via setImage) and offers a switch to
 * a Git repository (Render's Update Source repo↔image). A source change is never
 * auto-deployed — the next deploy uses the new source.
 */
export function ImageSourceCard({
  serviceId,
  imagePath,
  registryCredentialId,
}: {
  serviceId: string;
  imagePath: string;
  registryCredentialId: string | null;
}) {
  const { t } = useTranslations();
  const { setImage, busy } = useSetImage();
  const { disabled, reason } = useSourceKindGate();

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.sourceTitle")}</CardTitle>
        <CardDescription>{t("services.sourceImageDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <EditableFieldRow
          label={t("services.sourceImageLabel")}
          hint={t("services.sourceImageHint")}
          value={imagePath}
          placeholder={t("services.sourceImagePlaceholder")}
          editLabel={t("services.sourceImageEdit")}
          mono
          busy={busy}
          confirm={{
            title: (value) => t("services.sourceImageConfirmTitle", { value }),
            body: t("services.sourceNoAutoDeploy"),
          }}
          disabled={disabled}
          disabledReason={reason}
          onSave={(value) =>
            setImage(serviceId, value, registryCredentialId ?? undefined)
          }
        />
        <SwitchToRepoDialog serviceId={serviceId} disabled={disabled} />
      </CardContent>
    </Card>
  );
}

/**
 * SwitchToImageRow is the repo→image affordance appended to a **repo-backed**
 * service's Build section: a button that opens a dialog to enter a prebuilt
 * container image (+ optional registry credential), switching the service off
 * git. Render's Update Source, the direction the inline repo/branch editors
 * can't express.
 */
export function SwitchToImageRow({
  serviceId,
  disabled,
  disabledReason,
}: {
  serviceId: string;
  disabled: boolean;
  disabledReason?: string;
}) {
  const { t } = useTranslations();
  const { setImage, busy } = useSetImage();
  const [open, setOpen] = useState(false);
  const [image, setImageValue] = useState("");
  const [credentialId, setCredentialId] = useState("");
  const valid = image.trim() !== "";

  async function save() {
    const ok = await setImage(serviceId, image.trim(), credentialId || undefined);
    if (ok) {
      setOpen(false);
      setImageValue("");
      setCredentialId("");
    }
  }

  return (
    <div className="space-y-1">
      <div className="text-sm font-medium">{t("services.sourceSwitchLabel")}</div>
      <p className="text-sm text-muted-foreground">
        {t("services.sourceSwitchToImageHint")}
      </p>
      <Button
        variant="outline"
        size="sm"
        className="mt-2"
        disabled={disabled}
        onClick={() => setOpen(true)}
      >
        {t("services.sourceSwitchToImageButton")}
      </Button>
      {disabled && disabledReason && (
        <p className="text-sm text-muted-foreground">{disabledReason}</p>
      )}
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("services.sourceSwitchToImageButton")}</DialogTitle>
            <DialogDescription>
              {t("services.sourceNoAutoDeploy")}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="switch-image">
                {t("services.sourceImageLabel")}
              </Label>
              <Input
                id="switch-image"
                value={image}
                onChange={(e) => setImageValue(e.target.value)}
                placeholder={t("services.sourceImagePlaceholder")}
                autoComplete="off"
                className="font-mono"
              />
            </div>
            <RegistryCredentialSelect
              id="switch-image-credential"
              value={credentialId}
              onValueChange={setCredentialId}
              description={t("services.createRegistryCredentialDescription")}
            />
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setOpen(false)}>
              {t("services.buildDeployCancel")}
            </Button>
            <Button disabled={!valid || busy} onClick={() => void save()}>
              {busy && <Loader2 className="animate-spin" />}
              {t("services.sourceSwitchSave")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

/**
 * SwitchToRepoDialog is the image→repo affordance on the image-backed Source
 * card: a button opening a dialog to pick a repo from a connected account (or
 * paste a public git URL), switching the service to build-from-git via setRepo.
 */
function SwitchToRepoDialog({
  serviceId,
  disabled,
}: {
  serviceId: string;
  disabled: boolean;
}) {
  const { t } = useTranslations();
  const { setRepo, busy } = useSetRepo();
  const { repos } = useRepos();
  const [open, setOpen] = useState(false);
  const [url, setUrl] = useState("");
  const valid = isValidGitUrl(url);

  async function save() {
    const ok = await setRepo(serviceId, url.trim());
    if (ok) {
      setOpen(false);
      setUrl("");
    }
  }

  return (
    <div className="space-y-1">
      <div className="text-sm font-medium">{t("services.sourceSwitchLabel")}</div>
      <p className="text-sm text-muted-foreground">
        {t("services.sourceSwitchToRepoHint")}
      </p>
      <Button
        variant="outline"
        size="sm"
        className="mt-2"
        disabled={disabled}
        onClick={() => setOpen(true)}
      >
        {t("services.sourceSwitchToRepoButton")}
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("services.sourceSwitchToRepoButton")}</DialogTitle>
            <DialogDescription>
              {t("services.sourceNoAutoDeploy")}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            {repos.length > 0 && (
              <div className="max-h-56 overflow-y-auto rounded-md border divide-y">
                {repos.map((r) => (
                  <button
                    key={r.id}
                    type="button"
                    onClick={() => setUrl(r.htmlUrl)}
                    className={
                      "flex w-full items-center justify-between p-2.5 text-left text-sm transition-colors hover:bg-muted" +
                      (url === r.htmlUrl ? " bg-primary/5" : "")
                    }
                  >
                    <span className="truncate font-medium">{r.fullName}</span>
                    <span className="ml-3 shrink-0 text-xs text-muted-foreground">
                      {r.accountLogin}
                    </span>
                  </button>
                ))}
              </div>
            )}
            <div className="space-y-2">
              <Label htmlFor="switch-repo-url">
                {t("services.createPublicUrlLabel")}
              </Label>
              <Input
                id="switch-repo-url"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                placeholder={t("services.createPublicUrlPlaceholder")}
                autoComplete="off"
                className="font-mono"
              />
              {url && !valid && (
                <p className="text-sm text-destructive">
                  {t("services.createPublicUrlError")}
                </p>
              )}
            </div>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setOpen(false)}>
              {t("services.buildDeployCancel")}
            </Button>
            <Button disabled={!valid || busy} onClick={() => void save()}>
              {busy && <Loader2 className="animate-spin" />}
              {t("services.sourceSwitchSave")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
