import { useId, useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { Label } from "@/common/components/ui/label";
import { Switch } from "@/common/components/ui/switch";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCardRadioGroup } from "@/common/hooks/use-card-radio-group";
import { cn } from "@/common/lib/utils/utils";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import type { OwnerNotificationSettingsInput } from "@/graphql/definitions";
import {
  useOwnerNotificationSettings,
  type OwnerNotificationSettingsView,
} from "@/features/notifications/hooks/use-owner-notification-settings";
import {
  EMAIL_ROW_CLASS,
  LEVEL_OPTION_CLASS,
  WorkspaceNotificationsCardSkeleton,
} from "@/features/notifications/components/workspace-notifications-card-skeleton";

/** Render's owner levels, in Render's dashboard order. */
const LEVELS = [
  {
    level: "all",
    titleKey: "notifications.levelAll",
    hintKey: "notifications.levelAllHint",
  },
  {
    level: "failure",
    titleKey: "notifications.levelFailure",
    hintKey: "notifications.levelFailureHint",
  },
  {
    level: "none",
    titleKey: "notifications.levelNone",
    hintKey: "notifications.levelNoneHint",
  },
] as const;

const LEVEL_VALUES = LEVELS.map(({ level }) => level);

interface Draft {
  emailEnabled: boolean;
  notificationsToSend: string;
}

/** The fields of draft that differ from saved — the partial PATCH to send. */
function policyPatch(
  saved: OwnerNotificationSettingsView,
  draft: Draft,
): OwnerNotificationSettingsInput {
  const patch: OwnerNotificationSettingsInput = {};
  if (draft.emailEnabled !== saved.emailEnabled) {
    patch.emailEnabled = draft.emailEnabled;
  }
  if (draft.notificationsToSend !== saved.notificationsToSend) {
    patch.notificationsToSend = draft.notificationsToSend;
  }
  return patch;
}

/**
 * Workspace Settings → Notifications (w3/m271): the workspace notification
 * policy, Render's owner notification settings — whether email is sent at all
 * and which notifications a service on "Use workspace default" sends. Any
 * member sees it; only admins (can_manage) can change it.
 */
export function WorkspaceNotificationsCard({ ownerId }: { ownerId: string }) {
  const { t } = useTranslations();
  const { settings, loading, refetch, update, saving } =
    useOwnerNotificationSettings(ownerId);

  if (loading) return <WorkspaceNotificationsCardSkeleton />;
  if (!settings) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("notifications.workspaceTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          <Alert variant="destructive">
            <AlertDescription className="flex flex-wrap items-center gap-3">
              <span>{t("notifications.workspaceLoadFailed")}</span>
              <Button
                size="sm"
                variant="outline"
                onClick={() => void refetch()}
              >
                {t("notifications.workspaceRetry")}
              </Button>
            </AlertDescription>
          </Alert>
        </CardContent>
      </Card>
    );
  }
  // Keyed by workspace so switching workspaces re-seeds the draft.
  return (
    <WorkspaceNotificationsForm
      key={settings.ownerId}
      saved={settings}
      update={update}
      saving={saving}
    />
  );
}

interface FormProps {
  saved: OwnerNotificationSettingsView;
  update: (
    input: OwnerNotificationSettingsInput,
  ) => Promise<OwnerNotificationSettingsView>;
  saving: boolean;
}

function WorkspaceNotificationsForm({ saved, update, saving }: FormProps) {
  const { t } = useTranslations();
  const { canManage, denied } = useCapabilities();
  const emailId = `${useId()}-email`;
  const [draft, setDraft] = useState<Draft>(() => draftFrom(saved));
  // Re-seed from a newer server copy (a save, or another admin's change seen
  // on refetch) only while the form has no unsaved edits of its own.
  const [base, setBase] = useState(saved);
  if (base !== saved) {
    setBase(saved);
    if (Object.keys(policyPatch(base, draft)).length === 0) {
      setDraft(draftFrom(saved));
    }
  }
  const [failed, setFailed] = useState(false);

  const readOnly = !canManage;
  const patch = policyPatch(saved, draft);
  const canSave = !readOnly && Object.keys(patch).length > 0 && !saving;

  function edit(next: Partial<Draft>) {
    setDraft((prev) => ({ ...prev, ...next }));
    setFailed(false);
  }

  const radio = useCardRadioGroup(
    LEVEL_VALUES,
    draft.notificationsToSend,
    (level) => edit({ notificationsToSend: level }),
    readOnly,
  );

  async function handleSave() {
    if (!canSave) return;
    try {
      const next = await update(patch);
      setBase(next);
      setDraft(draftFrom(next));
      toast.success(t("notifications.workspaceSaved"));
    } catch {
      setFailed(true);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("notifications.workspaceTitle")}</CardTitle>
        <CardDescription>
          {t("notifications.workspaceDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        {denied("can_manage") ? (
          <p className="text-muted-foreground text-sm" role="note">
            {t("notifications.workspaceReadOnly")}
          </p>
        ) : null}

        <div className={EMAIL_ROW_CLASS}>
          <div className="space-y-0.5">
            <Label htmlFor={emailId}>
              {t("notifications.workspaceEmailLabel")}
            </Label>
            <p className="text-muted-foreground text-sm">
              {t("notifications.workspaceEmailHint")}
            </p>
          </div>
          <Switch
            id={emailId}
            checked={draft.emailEnabled}
            disabled={readOnly}
            onCheckedChange={(emailEnabled) => edit({ emailEnabled })}
          />
        </div>

        <div className="space-y-3">
          <div className="space-y-1">
            <h3 className="text-sm font-medium">
              {t("notifications.workspaceLevelLabel")}
            </h3>
            <p className="text-muted-foreground text-sm">
              {t("notifications.workspaceLevelHint")}
            </p>
          </div>
          <div
            role="radiogroup"
            aria-label={t("notifications.workspaceLevelLabel")}
            className="grid grid-cols-1 gap-3"
          >
            {LEVELS.map(({ level, titleKey, hintKey }) => {
              const checked = draft.notificationsToSend === level;
              return (
                <button
                  key={level}
                  {...radio(level)}
                  type="button"
                  role="radio"
                  aria-checked={checked}
                  disabled={readOnly}
                  onClick={() => edit({ notificationsToSend: level })}
                  className={cn(
                    LEVEL_OPTION_CLASS,
                    "text-left transition-colors disabled:cursor-not-allowed",
                    !readOnly && "hover:border-foreground/50",
                    checked
                      ? "border-primary ring-2 ring-primary"
                      : "border-border",
                  )}
                >
                  <span className="text-sm font-medium">{t(titleKey)}</span>
                  <span className="text-muted-foreground text-sm">
                    {t(hintKey)}
                  </span>
                </button>
              );
            })}
          </div>
        </div>

        {failed ? (
          <Alert variant="destructive">
            <AlertDescription>
              {t("notifications.workspaceSaveFailed")}
            </AlertDescription>
          </Alert>
        ) : null}
      </CardContent>
      <CardFooter className="justify-end border-t">
        <Button onClick={() => void handleSave()} disabled={!canSave}>
          {saving ? <Loader2 className="animate-spin" /> : null}
          {t("notifications.workspaceSave")}
        </Button>
      </CardFooter>
    </Card>
  );
}

function draftFrom(saved: OwnerNotificationSettingsView): Draft {
  return {
    emailEnabled: saved.emailEnabled,
    notificationsToSend: saved.notificationsToSend,
  };
}
