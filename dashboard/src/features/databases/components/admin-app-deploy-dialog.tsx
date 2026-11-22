import { useId, useState } from "react";
import { Eye, EyeOff } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { CopyButton } from "@/common/components/copy-button";
import { useTranslations } from "@/common/hooks/use-translations";
import { placementErrorMessage } from "@/features/environments/lib/placement";
import { isPaymentOnboardingCancelled } from "@/features/usage/context/payment-required-error";
import type { AdminAppView } from "@/features/databases/hooks/use-postgres-admin-apps";
import type { AdminAppCredentials } from "@/features/databases/lib/admin-apps";

export interface AdminAppDeployDialogProps {
  app: AdminAppView;
  /** The login the dialog opens with (Render's generated one). */
  initialCredentials: AdminAppCredentials;
  onOpenChange: (open: boolean) => void;
  deploy: (credentials: AdminAppCredentials) => Promise<{ name: string }>;
}

/**
 * Render's "Create a <app> app" dialog: its sentence on where the service
 * goes, the generated login (editable, masked, copyable) under
 * Authentication, the app's monthly charges, then Cancel and Deploy <app>.
 * A 402 opens billing onboarding and retries (the hook's payment gate); any
 * other refusal stays in the dialog, as Render's does.
 */
export function AdminAppDeployDialog({
  app,
  initialCredentials,
  onOpenChange,
  deploy,
}: AdminAppDeployDialogProps) {
  const { t } = useTranslations();
  const [credentials, setCredentials] = useState(initialCredentials);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const service = await deploy(credentials);
      toast.success(
        t("databases.adminAppDeployed", {
          app: app.name,
          service: service.name,
        }),
      );
      onOpenChange(false);
    } catch (err) {
      if (isPaymentOnboardingCancelled(err)) {
        setError(t("databases.adminAppPaymentRequired", { app: app.name }));
      } else {
        setError(
          placementErrorMessage(err, t("databases.adminAppDeployError"), t),
        );
      }
    } finally {
      setBusy(false);
    }
  };

  const planLabel = t(
    app.plan === "free"
      ? "databases.adminAppBillingFree"
      : "databases.adminAppBillingStarter",
  );

  return (
    <Dialog open onOpenChange={(open) => !busy && onOpenChange(open)}>
      <DialogContent className="sm:max-w-lg">
        <form className="space-y-6" noValidate onSubmit={submit}>
          <DialogHeader>
            <DialogTitle>
              {t("databases.adminAppDialogTitle", { app: app.name })}
            </DialogTitle>
            <DialogDescription>
              {t("databases.adminAppDialogDescription")}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            <div className="space-y-1">
              <p className="text-sm font-medium">
                {t("databases.adminAppAuthentication")}
              </p>
              <p className="text-sm text-muted-foreground">
                {t("databases.adminAppAuthenticationHint", { app: app.name })}
              </p>
            </div>
            <CredentialField
              label={t("databases.adminAppUsername")}
              value={credentials.username}
              onChange={(username) =>
                setCredentials((c) => ({ ...c, username }))
              }
            />
            <CredentialField
              label={t("databases.adminAppPassword")}
              value={credentials.password}
              onChange={(password) =>
                setCredentials((c) => ({ ...c, password }))
              }
            />
          </div>

          <div className="space-y-3">
            <div className="space-y-1">
              <p className="text-sm font-medium">
                {t("databases.adminAppMonthlyCharges")}
              </p>
              <p className="text-sm text-muted-foreground">
                {t("databases.adminAppMonthlyChargesHint", { app: app.name })}
              </p>
            </div>
            <ul className="divide-y rounded-md border text-sm">
              <BillingLine label={planLabel} usd={app.monthlyUsd} />
              {app.diskSizeGB != null && app.diskMonthlyUsd != null ? (
                <BillingLine
                  label={t("databases.adminAppBillingDisk", {
                    size: app.diskSizeGB,
                  })}
                  usd={app.diskMonthlyUsd}
                />
              ) : null}
            </ul>
          </div>

          {error ? (
            <Alert variant="destructive">
              <AlertTitle>{t("databases.adminAppErrorTitle")}</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              disabled={busy}
              onClick={() => onOpenChange(false)}
            >
              {t("common.cancel")}
            </Button>
            <Button type="submit" loading={busy}>
              {t("databases.adminAppDeploySubmit", { app: app.name })}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function BillingLine({ label, usd }: { label: string; usd: string }) {
  const { t } = useTranslations();
  return (
    <li className="flex items-center justify-between gap-4 px-4 py-3">
      <span>{label}</span>
      <span className="whitespace-nowrap tabular-nums">
        {t("databases.adminAppBillingPrice", { usd })}
      </span>
    </li>
  );
}

/** An editable login field, masked with a show toggle and a copy button. */
function CredentialField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  const { t } = useTranslations();
  const id = useId();
  const [visible, setVisible] = useState(false);
  return (
    <div className="grid gap-2 sm:grid-cols-[6rem_1fr] sm:items-center">
      <Label htmlFor={id}>{label}</Label>
      <div className="flex min-w-0 items-center gap-1">
        <Input
          id={id}
          type={visible ? "text" : "password"}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          autoComplete="off"
          spellCheck={false}
          className="font-mono"
        />
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          aria-label={t(
            visible ? "databases.adminAppHide" : "databases.adminAppShow",
            { field: label },
          )}
          aria-controls={id}
          onClick={() => setVisible((v) => !v)}
        >
          {visible ? <EyeOff /> : <Eye />}
        </Button>
        <CopyButton
          value={value}
          label={t("common.copyField", { field: label })}
          successText={t("databases.copied")}
          errorText={t("databases.copyError")}
        />
      </div>
    </div>
  );
}
