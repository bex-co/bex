import { useState, type ReactNode } from "react";
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
import {
  SANDBOX_GROUP_NAME_MAX,
  useCreateSandboxGroup,
  type SandboxGroup,
} from "@/features/sandbox-groups/hooks/use-create-sandbox-group";

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
 * A name. The region is not asked: bex-api places every group in the
 * platform's region (w32/016), which the confirmation shows as reported, the
 * way service settings show theirs. Render's optional project environment is
 * not offered because bex does not bind a group to one. Once created the
 * dialog confirms the group (id, name, region, whether it is the default, its
 * concurrency limit) in place, the way
 * the API-key dialog confirms a mint: Render documents no list or detail page
 * for groups, so there is nowhere truthful to navigate to.
 *
 * A duplicate name and the workspace's group limit stay inline.
 */
export function NewSandboxGroupDialog({
  open,
  onOpenChange,
}: NewSandboxGroupDialogProps) {
  const { t } = useTranslations();
  const { create, busy, refusal, clearRefusal } = useCreateSandboxGroup();
  const [name, setName] = useState("");
  const [created, setCreated] = useState<SandboxGroup | null>(null);

  // The name bex-api sees: it refuses a padded one, and every other create
  // dialog trims too.
  const trimmed = name.trim();
  const canSubmit = trimmed.length > 0 && !busy;
  const nameRefusal = refusal?.kind === "name" ? refusal.message : null;
  const limitRefusal = refusal?.kind === "limit" ? refusal.message : null;

  // Whatever closes it (Done, Cancel, Escape, the URL changing under it, the
  // browser's Back button) leaves a clean form for the next open, never a stale
  // confirmation. Adjusted during render, React's pattern for prop-driven resets.
  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (!open) {
      setName("");
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
    const group = await create(trimmed);
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
            <CreatedGroup group={created} />
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

/** The created group's facts, as a label/value list. */
function CreatedGroup({ group }: { group: SandboxGroup }) {
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
      <Row label={t("sandboxGroups.fieldRegion")}>
        {group.region}
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
