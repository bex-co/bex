import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { ExternalLink, Settings } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { useRootContext } from "@/common/hooks/use-root-context";
import { useTranslations } from "@/common/hooks/use-translations";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useEnvironments } from "@/features/environments/hooks/use-environments";
import { useProtectedPlacement } from "@/features/environments/hooks/use-protected-placement";
import { ServiceStatusBadge } from "@/features/services/components/service-status-badge";
import { deriveStatus } from "@/features/services/lib/status";
import {
  usePostgresAdminApps,
  type AdminAppView,
} from "@/features/databases/hooks/use-postgres-admin-apps";
import {
  ADMIN_APP_COPY,
  generateAdminAppCredentials,
  type AdminAppCredentials,
} from "@/features/databases/lib/admin-apps";
import { AdminAppDeployDialog } from "@/features/databases/components/admin-app-deploy-dialog";
import {
  ADMIN_APP_DESCRIPTION_CLASS,
  AdminAppsPanelSkeleton,
} from "@/features/databases/components/admin-apps-panel-skeleton";

/**
 * Render's Postgres "Apps" page as a section of the database page (w4/m551,
 * docs/render-artifacts/postgres-admin-apps.md): one card per admin app.
 * An undeployed app offers Deploy app, which opens the deploy dialog; a
 * deployed one shows its service's status, Open app (its URL, while it
 * serves) and Manage (its service page). Deploying needs the database's
 * connection (can_view_sensitive) and a create in its environment, which a
 * protected environment keeps to Admins; otherwise the button is disabled
 * with the reason.
 */
export function AdminAppsPanel({ databaseId }: { databaseId: string }) {
  const { t } = useTranslations();
  const { apps, projectId, environmentId, loading, error, deploy } =
    usePostgresAdminApps(databaseId);
  const { environments } = useEnvironments(projectId, { poll: false });
  const placement = useProtectedPlacement();
  const capabilities = useCapabilities();
  const { session } = useRootContext();
  const email = (session?.identity?.traits as { email?: string } | undefined)
    ?.email;
  const [dialog, setDialog] = useState<{
    app: AdminAppView;
    credentials: AdminAppCredentials;
  } | null>(null);

  if (loading) return <AdminAppsPanelSkeleton />;

  const environment = environments.find((e) => e.id === environmentId);
  const reasonKey = !capabilities.canViewSensitive
    ? capabilities.reasonKey("can_view_sensitive")
    : !capabilities.canCreate
      ? capabilities.reasonKey("can_create")
      : undefined;
  const deployReason = reasonKey
    ? t(reasonKey)
    : placement.restricts(environment)
      ? placement.reason
      : undefined;
  const deployable = capabilities.canViewSensitive && capabilities.canCreate;

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("databases.adminAppsTitle")}</CardTitle>
        <CardDescription>{t("databases.adminAppsDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        {error ? (
          <Alert variant="destructive">
            <AlertTitle>{t("databases.adminAppsErrorTitle")}</AlertTitle>
            <AlertDescription>
              {t("databases.adminAppsErrorBody")}
            </AlertDescription>
          </Alert>
        ) : (
          <div className="grid gap-4 md:grid-cols-2">
            {apps.map((app) => (
              <AdminAppCard
                key={app.id}
                app={app}
                deployReason={deployReason}
                deployDisabled={!deployable || deployReason !== undefined}
                onDeploy={() =>
                  setDialog({
                    app,
                    credentials: generateAdminAppCredentials(app.id, email),
                  })
                }
              />
            ))}
          </div>
        )}
      </CardContent>
      {dialog ? (
        <AdminAppDeployDialog
          app={dialog.app}
          initialCredentials={dialog.credentials}
          onOpenChange={(open) => !open && setDialog(null)}
          deploy={(credentials) => deploy(dialog.app.id, credentials)}
        />
      ) : null}
    </Card>
  );
}

function AdminAppCard({
  app,
  deployReason,
  deployDisabled,
  onDeploy,
}: {
  app: AdminAppView;
  deployReason: string | undefined;
  deployDisabled: boolean;
  onDeploy: () => void;
}) {
  const { t } = useTranslations();
  const copy = ADMIN_APP_COPY[app.id];
  const service = app.service;
  // Render disables Open app while the service deploys or cannot serve.
  const status = service ? deriveStatus(service).key : null;
  const serving = status === "running" || status === "sleeping";
  return (
    <div
      className="flex flex-col justify-between gap-4 rounded-lg border p-4"
      data-testid={`admin-app-${app.id}`}
    >
      <div className="space-y-2">
        <div className="flex min-h-8 items-center justify-between gap-2">
          <h3 className="flex min-w-0 items-center gap-2 text-base font-semibold">
            <span className="truncate">{app.name}</span>
            {service ? <ServiceStatusBadge service={service} /> : null}
          </h3>
          {service ? (
            <Button asChild variant="ghost" size="sm">
              <Link
                to="/services/$serviceId"
                params={{ serviceId: service.id }}
              >
                <Settings />
                {t("databases.adminAppManage")}
              </Link>
            </Button>
          ) : null}
        </div>
        <p className={ADMIN_APP_DESCRIPTION_CLASS}>{t(copy.descriptionKey)}</p>
      </div>
      <div className="flex flex-wrap items-center gap-3">
        {service ? (
          serving && service.url ? (
            <Button asChild size="sm">
              <a href={service.url} target="_blank" rel="noreferrer">
                {t("databases.adminAppOpen")}
                <ExternalLink />
              </a>
            </Button>
          ) : (
            <Button size="sm" disabled>
              {t("databases.adminAppOpen")}
              <ExternalLink />
            </Button>
          )
        ) : (
          <PermissionTooltip reason={deployReason}>
            <Button
              size="sm"
              variant="outline"
              disabled={deployDisabled}
              onClick={onDeploy}
            >
              {t("databases.adminAppDeploy")}
            </Button>
          </PermissionTooltip>
        )}
        {copy.docsUrl ? (
          <Button asChild variant="link" size="sm" className="px-0">
            <a href={copy.docsUrl} target="_blank" rel="noreferrer">
              {t("databases.adminAppDocs")}
              <ExternalLink />
            </a>
          </Button>
        ) : null}
      </div>
    </div>
  );
}
