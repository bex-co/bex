import { useId, useState } from "react";
import { Loader2, Plus } from "lucide-react";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/common/components/ui/dialog";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { LabelError } from "@/common/components/label-error";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCardRadioGroup } from "@/common/hooks/use-card-radio-group";
import {
  MAX_LABEL_LENGTH,
  isInvalidLabel,
  isValidLabel,
} from "@/common/lib/utils/resource-label";
import {
  DEFAULT_WORKSPACE_PRIVACY,
  WORKSPACE_PRIVACY_OPTIONS,
  workspacePlanNameKey,
  type WorkspacePrivacy,
} from "@/features/workspaces/types";
import { WorkspacePrivacyOption } from "@/features/workspaces/components/workspace-privacy-card-skeleton";
import { useCreateOrganizationWorkspace } from "@/features/organizations/hooks/use-organization-workspace-actions";

/**
 * Org Workspaces → + New Workspace (Render's organizations doc): "Provide a
 * name for the workspace", "Set the workspace's privacy setting to Public or
 * Invite-only", "Click Create Workspace". Invite-only is selected, as Render's
 * new workspaces are. There is no plan and no payment step: the workspace
 * takes the org's plan, which the dialog names instead of a price (ADR115 D2).
 */
export function CreateOrganizationWorkspaceDialog({
  organizationId,
  organizationName,
  plan,
}: {
  organizationId: string;
  organizationName: string;
  plan: string;
}) {
  const { t } = useTranslations();
  const nameId = useId();
  const privacyLabelId = useId();
  const messageId = useId();
  const { create, busy, refusal, clearRefusal } =
    useCreateOrganizationWorkspace(organizationId);
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [privacy, setPrivacy] = useState<WorkspacePrivacy>(
    DEFAULT_WORKSPACE_PRIVACY,
  );
  const radio = useCardRadioGroup(WORKSPACE_PRIVACY_OPTIONS, privacy, choose);
  const nameInvalid = isInvalidLabel(name);
  const planKey = workspacePlanNameKey(plan);

  function choose(option: WorkspacePrivacy) {
    setPrivacy(option);
    clearRefusal();
  }

  function reset() {
    setName("");
    setPrivacy(DEFAULT_WORKSPACE_PRIVACY);
    clearRefusal();
  }

  async function submit() {
    if (!isValidLabel(name) || busy) return;
    if (await create(name.trim(), privacy)) {
      setOpen(false);
      reset();
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) reset();
      }}
    >
      <DialogTrigger asChild>
        <Button size="sm">
          <Plus />
          {t("organizations.newWorkspace")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {t("organizations.createTitle", {
              organization: organizationName,
            })}
          </DialogTitle>
          <DialogDescription>
            {t("organizations.createDescription")}
          </DialogDescription>
        </DialogHeader>
        <form
          className="space-y-5"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <div className="space-y-2">
            <Label htmlFor={nameId}>{t("organizations.fieldName")}</Label>
            <Input
              id={nameId}
              value={name}
              maxLength={MAX_LABEL_LENGTH}
              autoComplete="off"
              autoFocus
              aria-invalid={nameInvalid || undefined}
              aria-describedby={nameInvalid ? messageId : undefined}
              onChange={(event) => {
                setName(event.target.value);
                clearRefusal();
              }}
            />
            <LabelError id={messageId} value={name} />
          </div>
          <div className="space-y-2">
            <p id={privacyLabelId} className="text-sm font-medium">
              {t("organizations.fieldPrivacy")}
            </p>
            <div
              role="radiogroup"
              aria-labelledby={privacyLabelId}
              className="grid grid-cols-1 gap-2"
            >
              {WORKSPACE_PRIVACY_OPTIONS.map((option) => {
                const { ref, ...keyboard } = radio(option);
                return (
                  <WorkspacePrivacyOption
                    key={option}
                    {...keyboard}
                    buttonRef={ref}
                    privacy={option}
                    checked={option === privacy}
                    onClick={() => choose(option)}
                  />
                );
              })}
            </div>
          </div>
          <div
            className="rounded-md border bg-muted/40 p-3 text-sm"
            data-testid="organization-plan-included"
          >
            <p className="font-medium">{t("organizations.planIncluded")}</p>
            <p className="text-muted-foreground">
              {t("organizations.planIncludedDetail", {
                plan: planKey ? t(planKey) : plan,
              })}
            </p>
          </div>
          {refusal ? (
            <p role="alert" className="text-sm text-destructive">
              {refusal}
            </p>
          ) : null}
          <DialogFooter>
            <DialogClose asChild>
              <Button type="button" variant="outline" disabled={busy}>
                {t("organizations.cancel")}
              </Button>
            </DialogClose>
            <Button type="submit" disabled={!isValidLabel(name) || busy}>
              {busy ? <Loader2 className="animate-spin" /> : null}
              {t("organizations.createSubmit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
