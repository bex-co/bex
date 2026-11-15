import { useEffect, useState, type ReactNode } from "react";
import { Loader2 } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { CopyButton } from "@/common/components/copy-button";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspaceEnvironmentIndex } from "@/features/env-groups/hooks/use-env-group-scope-index";
import { environmentNameIn } from "@/features/env-groups/lib/scope";
import { SandboxGroupEnvironmentField } from "@/features/sandbox-groups/components/sandbox-group-environment-field";
import {
  SANDBOX_GROUP_NAME_MAX,
  useCreateSandboxGroup,
  type SandboxGroup,
} from "@/features/sandbox-groups/hooks/use-create-sandbox-group";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export interface NewSandboxGroupDialogProps {
  /**
   * Controlled: the overview's URL-owned `?new=sandbox-group` (the "+ New"
   * menus' link target) drives it, so the dialog renders no trigger.
   */
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * "New Sandbox Group" from the "+ New" menu, as on Render (ADR104, w22/m311).
 * A name, and, for a workspace that has environments, Render's optional
 * project environment (w22/m412): the env group dialog's own select, on "No
 * environment" until one is chosen. The region is not asked: bex-api places
 * every group in the platform's region (w32/016), which the confirmation shows
 * as reported, the way service settings show theirs. Once created the dialog
 * confirms the group (id, name, region, environment, whether it is the
 * default, its concurrency limit) in place, the way the API-key dialog
 * confirms a mint: Render documents no list or detail page for groups, so
 * there is nowhere truthful to navigate to.
 *
 * The environments are read only while the dialog is open, and Create waits
 * for them so a group is never created in none by a click that beat the
 * select. A duplicate name, the workspace's group limit and a refused
 * environment stay inline.
 */
export function NewSandboxGroupDialog({
  open,
  onOpenChange,
}: NewSandboxGroupDialogProps) {
  const { t } = useTranslations();
  const { create, busy, refusal, clearRefusal } = useCreateSandboxGroup();
  const { currentWorkspaceId } = useWorkspace();
  const index = useWorkspaceEnvironmentIndex(open ? currentWorkspaceId : null);
  const [name, setName] = useState("");
  const [environmentChoice, setEnvironmentChoice] = useState<string | null>(
    null,
  );
  const [created, setCreated] = useState<SandboxGroup | null>(null);

  // The name bex-api sees: it refuses a padded one, and every other create
  // dialog trims too.
  const trimmed = name.trim();
  // The chosen environment, if the index still lists it: after a re-read that
  // dropped it the select shows No environment and none is sent.
  const environmentId = index.environments.some(
    (item) => item.id === environmentChoice,
  )
    ? environmentChoice
    : null;
  // Until the environments are read, or fail to be, the select is not there
  // to choose from; a failed read lets the group be created without one.
  const waitingForEnvironments = !index.ready && !index.error;
  const canSubmit = trimmed.length > 0 && !busy && !waitingForEnvironments;
  const nameRefusal = refusal?.kind === "name" ? refusal.message : null;
  const limitRefusal = refusal?.kind === "limit" ? refusal.message : null;
  const environmentRefusal =
    refusal?.kind === "environment" ? refusal.message : null;

  // A refused environment was chosen from a list that has since changed (it
  // was deleted, or its protection changed): read the list again.
  const environmentRefused = refusal?.kind === "environment";
  const rereadEnvironments = index.retry;
  useEffect(() => {
    if (environmentRefused) rereadEnvironments();
  }, [environmentRefused, rereadEnvironments]);

  // Whatever closes it (Done, Cancel, Escape, the URL changing under it, the
  // browser's Back button) leaves a clean form for the next open, never a stale
  // confirmation. Adjusted during render, React's pattern for prop-driven resets.
  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (!open) {
      setName("");
      setEnvironmentChoice(null);
      setCreated(null);
      clearRefusal();
    }
  }

  function handleOpenChange(next: boolean) {
    // A create in flight would land its result in a dialog that was closed and
    // reopened, so the dialog stays up until it answers.
    if (!next && busy) return;
    onOpenChange(next);
  }

  async function handleSubmit() {
    if (!canSubmit) return;
    const group = await create(trimmed, environmentId);
    if (group) setCreated(group);
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent>
        {created ? (
          <>
            <DialogHeader>
              <DialogTitle>{t("sandboxGroups.createdTitle")}</DialogTitle>
              <DialogDescription>
                {t("sandboxGroups.createdDescription", { name: created.name })}
              </DialogDescription>
            </DialogHeader>
            <CreatedGroup
              group={created}
              environmentName={
                created.environmentId
                  ? (environmentNameIn(index, created.environmentId) ??
                    created.environmentId)
                  : null
              }
            />
            <DialogFooter>
              <Button onClick={() => handleOpenChange(false)}>
                {t("sandboxGroups.createdDone")}
              </Button>
            </DialogFooter>
          </>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>{t("sandboxGroups.createTitle")}</DialogTitle>
              <DialogDescription>
                {t("sandboxGroups.createDescription")}
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-2">
              <Label htmlFor="sandbox-group-name">
                {t("sandboxGroups.fieldName")}
              </Label>
              <Input
                id="sandbox-group-name"
                value={name}
                maxLength={SANDBOX_GROUP_NAME_MAX}
                onChange={(e) => {
                  setName(e.target.value);
                  clearRefusal();
                }}
                placeholder={t("sandboxGroups.fieldNamePlaceholder")}
                autoComplete="off"
                autoFocus
                aria-invalid={nameRefusal != null}
                aria-describedby={
                  nameRefusal ? "sandbox-group-name-refusal" : undefined
                }
                onKeyDown={(e) => {
                  if (e.key === "Enter") void handleSubmit();
                }}
              />
              {nameRefusal ? (
                <p
                  id="sandbox-group-name-refusal"
                  role="alert"
                  className="text-destructive text-sm"
                >
                  {nameRefusal}
                </p>
              ) : null}
            </div>

            <SandboxGroupEnvironmentField
              index={index}
              value={environmentId}
              onValueChange={(next) => {
                setEnvironmentChoice(next);
                clearRefusal();
              }}
              disabled={busy}
              refusal={environmentRefusal}
            />

            {limitRefusal ? (
              <Alert variant="destructive">
                <AlertTitle>{t("sandboxGroups.errorLimitTitle")}</AlertTitle>
                <AlertDescription>{limitRefusal}</AlertDescription>
              </Alert>
            ) : null}

            <DialogFooter>
              <Button
                variant="outline"
                onClick={() => handleOpenChange(false)}
                disabled={busy}
              >
                {t("sandboxGroups.createCancel")}
              </Button>
              <Button onClick={() => void handleSubmit()} disabled={!canSubmit}>
                {busy ? <Loader2 className="animate-spin" /> : null}
                {t("sandboxGroups.createSubmit")}
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

/**
 * The created group's facts, as a label/value list. `environmentName` is the
 * environment it was placed in, null for a group in none.
 */
function CreatedGroup({
  group,
  environmentName,
}: {
  group: SandboxGroup;
  environmentName: string | null;
}) {
  const { t } = useTranslations();
  return (
    <dl className="space-y-3">
      <Row label={t("sandboxGroups.fieldName")}>
        <span className="break-all">{group.name}</span>
      </Row>
      <Row label={t("sandboxGroups.fieldId")}>
        <div className="flex items-center gap-1">
          <code className="font-mono break-all">{group.id}</code>
          <CopyButton
            value={group.id}
            label={t("sandboxGroups.copyId")}
            successText={t("sandboxGroups.copied")}
            errorText={t("sandboxGroups.copyError")}
          />
        </div>
      </Row>
      <Row label={t("sandboxGroups.fieldRegion")}>{group.region}</Row>
      <Row label={t("envGroups.environmentLabel")}>
        <span className="break-all">
          {environmentName ?? t("environments.assignmentEnvironmentNone")}
        </span>
      </Row>
      <Row label={t("sandboxGroups.fieldDefault")}>
        {group.isDefault
          ? t("sandboxGroups.defaultYes")
          : t("sandboxGroups.defaultNo")}
      </Row>
      <Row label={t("sandboxGroups.fieldConcurrencyLimit")}>
        <span className="tabular-nums">{group.concurrencyLimit}</span>
      </Row>
    </dl>
  );
}

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 border-b pb-2 last:border-0">
      <dt className="text-muted-foreground text-sm">{label}</dt>
      <dd className="min-w-0 text-sm font-medium">{children}</dd>
    </div>
  );
}
