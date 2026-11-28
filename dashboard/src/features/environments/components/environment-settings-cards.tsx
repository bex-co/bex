import { useId, useRef, useState } from "react";
import { Link, useNavigate, useRouter } from "@tanstack/react-router";
import { Info, Loader2, Pencil, Trash2 } from "lucide-react";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Input } from "@/common/components/ui/input";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { LabelError } from "@/common/components/label-error";
import {
  PLACEHOLDER_SENTINEL,
  aroundPlaceholder,
} from "@/common/lib/i18n-placeholder";
import {
  isNameConflictError,
  mutationErrorMessage,
} from "@/common/lib/graphql-error";
import { isInvalidLabel } from "@/common/lib/utils/resource-label";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import {
  useUpdateEnvironment,
  type EnvironmentPatch,
} from "@/features/environments/hooks/use-update-environment";
import { useDeleteEnvironment } from "@/features/environments/hooks/use-delete-environment";
import { IsolationConfirmDialog } from "@/features/environments/components/isolation-confirm-dialog";
import {
  CHOICE_RADIO_CLASS,
  ChoiceCardBody,
  ChoiceOptionRow,
  EDIT_FOOTER_CLASS,
  PlanGateRow,
} from "@/features/environments/components/environment-settings-layout";
import {
  CONNECTION_OPTIONS,
  PROTECTION_OPTIONS,
  type ChoiceOptionCopy,
  type Connections,
  type Protection,
} from "@/features/environments/lib/settings";
import { UpgradePlanLink } from "@/features/workspaces/components/upgrade-plan-link";

/** The projects and environments docs, Render's "protected environment" link. */
const ENVIRONMENTS_DOCS_URL =
  "https://bex.co/docs/platform/projects-environments";

/**
 * The Cancel / Save row an editing card ends with (Render's settings cards).
 */
function EditActions({
  saving,
  saveDisabled,
  onCancel,
}: {
  saving: boolean;
  saveDisabled: boolean;
  onCancel: () => void;
}) {
  const { t } = useTranslations();
  return (
    <div className="flex justify-end gap-2">
      <Button type="button" variant="outline" onClick={onCancel}>
        {t("environments.cancel")}
      </Button>
      <Button type="submit" disabled={saving || saveDisabled}>
        {saving ? <Loader2 className="animate-spin" /> : null}
        {t("environments.settingsCardSave")}
      </Button>
    </div>
  );
}

function EditButton({
  reason,
  onClick,
  variant = "outline",
}: {
  reason?: string;
  onClick: () => void;
  variant?: "outline" | "ghost";
}) {
  const { t } = useTranslations();
  return (
    <PermissionTooltip reason={reason}>
      <Button
        type="button"
        variant={variant}
        size="sm"
        disabled={reason !== undefined}
        onClick={onClick}
      >
        <Pencil />
        {t("environments.settingsEdit")}
      </Button>
    </PermissionTooltip>
  );
}

/**
 * Render's Name card: the name read-only until Edit, then Cancel and Save,
 * with Render's "Environment name is required" and "Name is already used by
 * another environment in this project". A role that cannot rename sees Edit
 * disabled with the reason.
 */
export function EnvironmentNameCard({
  environment,
  reason,
}: {
  environment: EnvironmentView;
  /** Why the caller may not rename it, when they may not. */
  reason?: string;
}) {
  const { t } = useTranslations();
  const fieldId = useId();
  const errorId = `${fieldId}-error`;
  const inputRef = useRef<HTMLInputElement>(null);
  const router = useRouter();
  const { update, saving } = useUpdateEnvironment();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(environment.name);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [cardError, setCardError] = useState<string | null>(null);

  function startEdit() {
    setDraft(environment.name);
    setFieldError(null);
    setCardError(null);
    setEditing(true);
    inputRef.current?.focus();
  }

  function cancel() {
    setEditing(false);
    setFieldError(null);
    setCardError(null);
  }

  async function save() {
    const name = draft.trim();
    setCardError(null);
    if (name === "") {
      setFieldError(t("environments.nameRequired"));
      return;
    }
    if (isInvalidLabel(draft)) return;
    setFieldError(null);
    const result = await update(environment.id, { name });
    if (result.ok) {
      setEditing(false);
      // The document title (`<project> / <env> / Settings`) is the loader's.
      void router.invalidate();
    } else if (isNameConflictError(result.error)) {
      setFieldError(t("environments.nameInUse"));
    } else {
      setCardError(
        mutationErrorMessage(
          result.error,
          t("environments.settingsUpdateError"),
        ),
      );
    }
  }

  const invalidLabel = editing && isInvalidLabel(draft);
  return (
    <Card>
      <form
        noValidate
        onSubmit={(event) => {
          event.preventDefault();
          void save();
        }}
        className="contents"
      >
        <CardHeader>
          <CardTitle>{t("environments.nameCardTitle")}</CardTitle>
          <CardDescription>
            {t("environments.nameCardDescription")}
          </CardDescription>
          {editing ? null : (
            <CardAction>
              <EditButton reason={reason} onClick={startEdit} />
            </CardAction>
          )}
        </CardHeader>
        <CardContent className="space-y-2">
          <label htmlFor={fieldId} className="sr-only">
            {t("environments.nameFieldLabel")}
          </label>
          <Input
            ref={inputRef}
            id={fieldId}
            className="max-w-[35rem]"
            value={editing ? draft : environment.name}
            readOnly={!editing}
            onChange={(event) => {
              setDraft(event.target.value);
              setFieldError(null);
            }}
            onKeyDown={(event) => {
              if (event.key === "Escape" && editing) cancel();
            }}
            autoComplete="off"
            aria-invalid={fieldError !== null || invalidLabel}
            aria-describedby={
              fieldError !== null || invalidLabel ? errorId : undefined
            }
          />
          {fieldError !== null ? (
            <p id={errorId} role="alert" className="text-xs text-destructive">
              {fieldError}
            </p>
          ) : editing ? (
            <LabelError id={errorId} value={draft} />
          ) : null}
          {cardError ? <CardError message={cardError} /> : null}
        </CardContent>
        {editing ? (
          <div className={EDIT_FOOTER_CLASS}>
            <EditActions
              saving={saving}
              saveDisabled={draft === environment.name || invalidLabel}
              onCancel={cancel}
            />
          </div>
        ) : null}
      </form>
    </Card>
  );
}

function CardError({ message }: { message: string }) {
  return (
    <Alert variant="destructive" role="alert">
      <AlertDescription>{message}</AlertDescription>
    </Alert>
  );
}

/**
 * Render's two-option radio list on the Permissions and Cross-Environment
 * Connections cards, disabled until Edit.
 */
function ChoiceOptions<T extends string>({
  legend,
  options,
  value,
  disabled,
  onChange,
}: {
  legend: string;
  options: Array<ChoiceOptionCopy<T>>;
  value: T;
  disabled: boolean;
  onChange: (value: T) => void;
}) {
  const { t } = useTranslations();
  const name = useId();
  return (
    <fieldset className="space-y-2" disabled={disabled}>
      <legend className="sr-only">{legend}</legend>
      {options.map((option) => {
        const id = `${name}-${option.value}`;
        return (
          <ChoiceOptionRow
            key={option.value}
            htmlFor={id}
            selected={value === option.value}
            radio={
              <input
                id={id}
                type="radio"
                name={name}
                value={option.value}
                checked={value === option.value}
                onChange={() => onChange(option.value)}
                className={cn(
                  CHOICE_RADIO_CLASS,
                  "accent-primary cursor-pointer disabled:cursor-default",
                )}
              />
            }
            title={t(option.titleKey)}
            icon={option.icon}
            hint={t(option.hintKey)}
          />
        );
      })}
    </fieldset>
  );
}

/**
 * The Edit → choose → (confirm) → Save flow both choice cards share. Render
 * returns a card to Edit on success and keeps it editing with the refusal on
 * the card otherwise.
 */
function useChoiceEdit<T extends string>(
  environmentId: string,
  current: T,
  patch: (value: T) => EnvironmentPatch,
) {
  const { t } = useTranslations();
  const { update, saving } = useUpdateEnvironment();
  const [editing, setEditing] = useState(false);
  const [choice, setChoice] = useState<T>(current);
  const [confirming, setConfirming] = useState(false);
  const [error, setError] = useState<string | null>(null);

  return {
    editing,
    choice,
    setChoice,
    shown: editing ? choice : current,
    confirming,
    setConfirming,
    error,
    saving,
    startEdit() {
      setChoice(current);
      setError(null);
      setEditing(true);
    },
    cancel() {
      setEditing(false);
      setConfirming(false);
      setChoice(current);
      setError(null);
    },
    async save() {
      setConfirming(false);
      setError(null);
      const result = await update(environmentId, patch(choice));
      if (result.ok) setEditing(false);
      else
        setError(
          mutationErrorMessage(
            result.error,
            t("environments.settingsUpdateError"),
          ),
        );
    },
  };
}

/** A choice card's Cancel · Save while editing, else its Edit. */
function ChoiceActions({
  edit,
  unchanged,
}: {
  edit: {
    editing: boolean;
    saving: boolean;
    cancel: () => void;
    startEdit: () => void;
  };
  unchanged: boolean;
}) {
  return edit.editing ? (
    <EditActions
      saving={edit.saving}
      saveDisabled={unchanged}
      onCancel={edit.cancel}
    />
  ) : (
    <EditButton variant="ghost" onClick={edit.startEdit} />
  );
}

/**
 * Render's Permissions card: Default ("For staging and development") or
 * Protected ("Best for production"). Saving over a protected environment
 * first asks "Remove restrictions". The page shows the card only to an Admin,
 * as Render shows it only to a member who may change the protection.
 */
export function EnvironmentPermissionsCard({
  environment,
}: {
  environment: EnvironmentView;
}) {
  const { t } = useTranslations();
  const current: Protection =
    environment.protectedStatus === "protected" ? "protected" : "unprotected";
  const edit = useChoiceEdit(environment.id, current, (protectedStatus) => ({
    protectedStatus,
  }));
  const [before, after] = aroundPlaceholder(
    t("environments.permissionsProtectedBody", {
      protectedEnvironment: PLACEHOLDER_SENTINEL,
    }),
  );

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("environments.permissionsCardTitle")}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <form
          noValidate
          onSubmit={(event) => {
            event.preventDefault();
            if (current === "protected") edit.setConfirming(true);
            else void edit.save();
          }}
        >
          <ChoiceCardBody
            explanation={
              <>
                <p>{t("environments.permissionsDefaultBody")}</p>
                <p>
                  {before}
                  <a
                    href={ENVIRONMENTS_DOCS_URL}
                    target="_blank"
                    rel="noreferrer"
                    className="font-medium text-foreground underline underline-offset-4"
                  >
                    {t("environments.permissionsProtectedLink")}
                  </a>
                  {after}
                </p>
              </>
            }
            choice={
              <ChoiceOptions
                legend={t("environments.permissionsLegend")}
                options={PROTECTION_OPTIONS}
                value={edit.shown}
                disabled={!edit.editing}
                onChange={edit.setChoice}
              />
            }
            actions={
              <ChoiceActions edit={edit} unchanged={edit.choice === current} />
            }
          />
        </form>
        {edit.error ? <CardError message={edit.error} /> : null}
      </CardContent>
      <ConfirmDialog
        open={edit.confirming}
        onOpenChange={(open) => {
          if (!open) edit.setConfirming(false);
        }}
        title={t("environments.removeRestrictionsTitle")}
        description={t("environments.removeProtectionBody", {
          name: environment.name,
        })}
        cancelLabel={t("environments.cancel")}
        confirmLabel={t("environments.removeRestrictionsConfirm")}
        onConfirm={() => void edit.save()}
      />
    </Card>
  );
}

/**
 * Render's Cross-Environment Connections card: Allowed ("Default for new
 * environments") or Blocked ("Recommended for production environments"),
 * confirmed in Render's isolation dialog, which closes as the save starts and
 * leaves a refusal on the card, as Render's card does. `gated` is Render's plan
 * gate (below Pro, unless the environment is already isolated).
 */
export function EnvironmentConnectionsCard({
  environment,
  projectName,
  gated,
}: {
  environment: EnvironmentView;
  projectName: string;
  gated: boolean;
}) {
  const { t } = useTranslations();
  const current: Connections = environment.networkIsolationEnabled
    ? "blocked"
    : "allowed";
  const edit = useChoiceEdit(environment.id, current, (choice) => ({
    networkIsolationEnabled: choice === "blocked",
  }));

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("environments.connectionsCardTitle")}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <form
          noValidate
          onSubmit={(event) => {
            event.preventDefault();
            edit.setConfirming(true);
          }}
        >
          <ChoiceCardBody
            explanation={
              <>
                <p>{t("environments.connectionsBody")}</p>
                <p>{t("environments.connectionsBodyOutside")}</p>
              </>
            }
            choice={
              <ChoiceOptions
                legend={t("environments.connectionsLegend")}
                options={CONNECTION_OPTIONS}
                value={edit.shown}
                disabled={!edit.editing}
                onChange={edit.setChoice}
              />
            }
            actions={
              gated ? null : (
                <ChoiceActions
                  edit={edit}
                  unchanged={edit.choice === current}
                />
              )
            }
          />
        </form>
        {edit.error ? <CardError message={edit.error} /> : null}
        {gated ? (
          <PlanGateRow
            text={t("environments.connectionsPaywall", {
              plan: t("workspaces.planProName"),
            })}
            action={
              <Button asChild size="sm">
                <UpgradePlanLink plan="pro">
                  {t("environments.upgrade")}
                </UpgradePlanLink>
              </Button>
            }
          />
        ) : null}
      </CardContent>
      <IsolationConfirmDialog
        open={edit.confirming}
        onOpenChange={(open) => {
          if (!open) edit.setConfirming(false);
        }}
        restrict={edit.choice === "blocked"}
        projectName={projectName}
        environmentName={environment.name}
        pending={edit.saving}
        error={null}
        onConfirm={() => void edit.save()}
      />
    </Card>
  );
}

/**
 * Render's Delete Environment card: its sentence and a Delete environment
 * button that asks for `sudo delete environment <name>`. A project's only
 * environment cannot be deleted: Render's notice points at deleting the
 * project instead, and the button stays disabled. After the delete the browser
 * lands on the project's settings, as Render's does. A member who may not
 * delete it sees Render's "Environment Deletion" card with the reason instead.
 */
export function EnvironmentDeleteCard({
  environment,
  onlyEnvironment,
  deniedReason,
}: {
  environment: EnvironmentView;
  onlyEnvironment: boolean;
  deniedReason?: string;
}) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { projectId } = environment;
  const { remove, deleting } = useDeleteEnvironment(projectId);
  const [open, setOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const busy = deleting === environment.id;

  if (deniedReason !== undefined) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("environments.deleteCardDenied")}</CardTitle>
          <CardDescription>{deniedReason}</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  async function confirm() {
    setError(null);
    const result = await remove(environment.id, environment.name);
    if (result.ok) {
      setOpen(false);
      void navigate({
        to: "/project/$projectId/settings",
        params: { projectId },
        replace: true,
      });
    } else {
      setError(result.message || t("environments.deleteFailed"));
    }
  }

  const [before, after] = aroundPlaceholder(
    t("environments.deleteLastEnvironment", { project: PLACEHOLDER_SENTINEL }),
  );
  // bex detaches the environment's env groups; the dialog says so.
  const keptGroups =
    environment.envGroupIds.length > 0 ? (
      <p className="text-sm text-muted-foreground">
        {t("environments.deleteDialogEnvGroupsKept")}
      </p>
    ) : null;

  return (
    <Card className="border-destructive/50">
      <CardHeader>
        <CardTitle>{t("environments.deleteCardTitle")}</CardTitle>
        <CardDescription>
          {t("environments.deleteCardDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        {onlyEnvironment ? (
          <Alert>
            <Info />
            <AlertDescription>
              <p>
                {before}
                <Link
                  to="/project/$projectId/settings"
                  params={{ projectId }}
                  className="font-medium text-foreground underline underline-offset-4"
                >
                  {t("environments.deleteLastEnvironmentProject")}
                </Link>
                {after}
              </p>
            </AlertDescription>
          </Alert>
        ) : null}
        <Button
          variant="destructive"
          disabled={onlyEnvironment}
          onClick={() => {
            setError(null);
            setOpen(true);
          }}
        >
          <Trash2 />
          {t("environments.deleteButton")}
        </Button>
      </CardContent>
      <ConfirmDialog
        open={open}
        onOpenChange={(next) => {
          if (!busy) setOpen(next);
        }}
        title={t("environments.deleteButton")}
        description={t("environments.deleteDialogBody")}
        phrase={`sudo delete environment ${environment.name}`}
        cancelLabel={t("environments.cancel")}
        confirmLabel={
          <>
            {busy ? <Loader2 className="animate-spin" /> : null}
            {t("environments.deleteButton")}
          </>
        }
        pending={busy}
        closeOnConfirm={false}
        onConfirm={() => void confirm()}
      >
        {keptGroups}
        {error ? <CardError message={error} /> : null}
      </ConfirmDialog>
    </Card>
  );
}
