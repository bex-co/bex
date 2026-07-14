import { useState } from "react";
import { Pencil, Check, X, Loader2 } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Switch } from "@/common/components/ui/switch";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/common/components/ui/alert-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { useRootDir } from "@/features/services/hooks/use-root-dir";
import { usePreDeployCommand } from "@/features/services/hooks/use-pre-deploy-command";
import { useAutoDeploy } from "@/features/services/hooks/use-auto-deploy";
import { useGitConnection } from "@/features/git/hooks/use-git-connection";

export interface BuildDeploySectionProps {
  serviceId: string;
  /** spec.repo — always set for a build-from-git App (this section only renders then). */
  repo: string;
  /** spec.branch, or null while loading. */
  branch: string | null;
  /** spec.rootDir; empty/null builds from the repo root. */
  rootDir: string | null;
  /** spec.autoDeploy — whether a signed git push redeploys this App. */
  autoDeploy: boolean;
  /** spec.preDeployCommand; empty/null means no pre-deploy step (w1/m33). */
  preDeployCommand: string | null;
  /**
   * Whether to show the Pre-Deploy Command field — true for web/private/worker,
   * false for cron_job/static_site (the pre-deploy step doesn't apply there, and
   * the backend rejects setPreDeployCommand for them).
   */
  showPreDeployCommand: boolean;
}

/**
 * The Settings tab's "Build & Deploy" section (w5/m13, Render parity — layout
 * captured live from Render's own Settings → Build panel,
 * `.playwright-mcp/render-build-deploy-settings.png`): Source + Branch
 * read-only (bex has no write path for them yet), Root Directory editable
 * inline (pencil → input → confirm, following `w5/m7`'s plan-picker confirm
 * pattern since a change triggers a rebuild). Only rendered for a
 * build-from-git App — an image-backed App has nothing to build.
 */
export function BuildDeploySection({
  serviceId,
  repo,
  branch,
  rootDir,
  autoDeploy,
  preDeployCommand,
  showPreDeployCommand,
}: BuildDeploySectionProps) {
  const { t } = useTranslations();
  const { setRootDir, busy } = useRootDir();
  const { setPreDeployCommand, busy: preDeployBusy } = usePreDeployCommand();
  const { setAutoDeploy, busy: autoDeployBusy } = useAutoDeploy();
  const { connection } = useGitConnection();
  // Optimistic switch state — reverted on a failed mutation.
  const [autoDeployOn, setAutoDeployOn] = useState(autoDeploy);
  // A linear flow (view -> editing -> confirming), modeled as one enum rather
  // than two independent booleans so "editing but not confirming" and
  // "confirming" can't drift out of sync.
  const [mode, setMode] = useState<"view" | "editing" | "confirming">("view");
  const [draft, setDraft] = useState("");
  // Pre-deploy command inline edit (w1/m33): a plain pencil→input→save flow, no
  // confirm dialog — Render edits this field inline with a Save button, and it
  // has its own state so it can't collide with the Root Directory edit above.
  const [preMode, setPreMode] = useState<"view" | "editing">("view");
  const [preDraft, setPreDraft] = useState("");

  // A repo hosted on the connected GitHub account auto-deploys hands-free via
  // the app's app-wide webhook; otherwise a push needs the manual HMAC webhook.
  // (The backend does the precise repo-grant match; this is the UI hint.)
  const viaGitHub = !!connection?.connected && /github\.com[/:]/i.test(repo);

  async function handleAutoDeployChange(next: boolean) {
    setAutoDeployOn(next);
    const ok = await setAutoDeploy(serviceId, next);
    if (!ok) setAutoDeployOn(!next); // revert
  }

  const current = rootDir ?? "";
  const canSave = draft.trim() !== current;

  function startEdit() {
    setDraft(current);
    setMode("editing");
  }

  async function handleConfirm() {
    setMode("editing");
    const ok = await setRootDir(serviceId, draft.trim());
    if (ok) setMode("view");
  }

  const preCurrent = preDeployCommand ?? "";
  const preCanSave = preDraft.trim() !== preCurrent;

  function startPreEdit() {
    setPreDraft(preCurrent);
    setPreMode("editing");
  }

  async function handlePreSave() {
    const ok = await setPreDeployCommand(serviceId, preDraft.trim());
    if (ok) setPreMode("view");
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.buildDeployTitle")}</CardTitle>
        <CardDescription>
          {t("services.buildDeployDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <div>
          <div className="text-sm text-muted-foreground">
            {t("services.buildDeploySourceLabel")}
          </div>
          <div className="mt-1 font-mono text-sm break-all">{repo}</div>
        </div>
        <div>
          <div className="text-sm text-muted-foreground">
            {t("services.buildDeployBranchLabel")}
          </div>
          <div className="mt-1 font-mono text-sm">{branch || "—"}</div>
        </div>
        <div>
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            {t("services.buildDeployRootDirLabel")}
            <Badge variant="outline" className="text-xs font-normal">
              {t("services.buildDeployRootDirOptional")}
            </Badge>
          </div>
          <div className="mt-1 text-sm text-muted-foreground">
            {t("services.buildDeployRootDirHint")}
          </div>
          {mode !== "view" ? (
            <div className="mt-2 flex items-center gap-2">
              <Input
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                placeholder={t("services.buildDeployRootDirPlaceholder")}
                autoFocus
                className="font-mono text-sm"
                onKeyDown={(e) => {
                  if (e.key === "Enter" && canSave) setMode("confirming");
                  if (e.key === "Escape") setMode("view");
                }}
              />
              <Button
                size="icon"
                variant="ghost"
                aria-label={t("services.buildDeploySave")}
                disabled={busy || !canSave}
                onClick={() => setMode("confirming")}
              >
                {busy ? (
                  <Loader2 className="animate-spin" />
                ) : (
                  <Check className="text-emerald-600" />
                )}
              </Button>
              <Button
                size="icon"
                variant="ghost"
                aria-label={t("services.buildDeployCancel")}
                disabled={busy}
                onClick={() => setMode("view")}
              >
                <X />
              </Button>
            </div>
          ) : (
            <div className="mt-2 flex items-center gap-2">
              {current ? (
                <span className="font-mono text-sm">{current}</span>
              ) : (
                <span className="text-sm text-muted-foreground italic">
                  {t("services.buildDeployRootDirEmpty")}
                </span>
              )}
              <Button
                size="icon"
                variant="ghost"
                aria-label={t("services.buildDeployEdit")}
                onClick={startEdit}
              >
                <Pencil />
              </Button>
            </div>
          )}
        </div>

        {showPreDeployCommand && (
          <div>
            <div className="text-sm text-muted-foreground">
              {t("services.preDeployLabel")}
            </div>
            <div className="mt-1 text-sm text-muted-foreground">
              {t("services.preDeployHint")}
            </div>
            {preMode === "editing" ? (
              <div className="mt-2 flex items-center gap-2">
                <Input
                  value={preDraft}
                  onChange={(e) => setPreDraft(e.target.value)}
                  placeholder={t("services.preDeployPlaceholder")}
                  autoFocus
                  className="font-mono text-sm"
                  onKeyDown={(e) => {
                    if (e.key === "Enter" && preCanSave) void handlePreSave();
                    if (e.key === "Escape") setPreMode("view");
                  }}
                />
                <Button
                  size="icon"
                  variant="ghost"
                  aria-label={t("services.buildDeploySave")}
                  disabled={preDeployBusy || !preCanSave}
                  onClick={() => void handlePreSave()}
                >
                  {preDeployBusy ? (
                    <Loader2 className="animate-spin" />
                  ) : (
                    <Check className="text-emerald-600" />
                  )}
                </Button>
                <Button
                  size="icon"
                  variant="ghost"
                  aria-label={t("services.buildDeployCancel")}
                  disabled={preDeployBusy}
                  onClick={() => setPreMode("view")}
                >
                  <X />
                </Button>
              </div>
            ) : (
              <div className="mt-2 flex items-center gap-2">
                {preCurrent ? (
                  <span className="font-mono text-sm break-all">
                    {preCurrent}
                  </span>
                ) : (
                  <span className="text-sm text-muted-foreground italic">
                    {t("services.preDeployEmpty")}
                  </span>
                )}
                <Button
                  size="icon"
                  variant="ghost"
                  aria-label={t("services.preDeployEdit")}
                  onClick={startPreEdit}
                >
                  <Pencil />
                </Button>
              </div>
            )}
          </div>
        )}

        <div className="flex items-start justify-between gap-4">
          <div className="space-y-1">
            <div className="text-sm text-muted-foreground">
              {t("services.autoDeployLabel")}
            </div>
            <div className="text-sm text-muted-foreground">
              {viaGitHub
                ? t("services.autoDeployViaGitHub")
                : t("services.autoDeployViaWebhook")}
            </div>
          </div>
          <Switch
            checked={autoDeployOn}
            disabled={autoDeployBusy}
            onCheckedChange={handleAutoDeployChange}
            aria-label={t("services.autoDeployLabel")}
          />
        </div>
      </CardContent>

      <AlertDialog
        open={mode === "confirming"}
        onOpenChange={(open) => setMode(open ? "confirming" : "editing")}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("services.buildDeployConfirmTitle", {
                value: draft.trim() || t("services.buildDeployConfirmRoot"),
              })}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("services.buildDeployConfirmBody")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>
              {t("services.buildDeployCancel")}
            </AlertDialogCancel>
            <AlertDialogAction onClick={() => void handleConfirm()}>
              {t("services.buildDeploySave")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}
