import { Link } from "@tanstack/react-router";
import { KeyRound, SquareTerminal, Terminal } from "lucide-react";
import { ConnectionField } from "@/common/components/connection-field";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { ServiceShellCardsSkeleton } from "@/common/components/route-skeletons";
import { useTranslations } from "@/common/hooks/use-translations";
import { AddSshKeyCta } from "@/features/ssh-keys/components/add-ssh-key-cta";
import { RequiresSshKey } from "@/features/ssh-keys/components/requires-ssh-key";
import { useServer } from "@/features/services/hooks/use-server";
import { WebShellPanel } from "@/features/services/components/web-shell-panel";

/**
 * Render's Shell page hosts an in-browser terminal alongside the copy-ready SSH
 * command; bex matches this (w2/m55, docs/ADR035-ssh.md § Browser Web Shell).
 * The terminal streams only over the isolated gateway's authenticated
 * WebSocket, gated by a short-lived can_operate exec ticket — no SSH private key
 * or Kubernetes exec credential ever reaches the browser, and bex-api never
 * gains pods/exec. `sshAddress` presence is the eligibility signal both paths
 * share (paid, non-suspended web/private/worker with the gateway configured).
 */
export function ServiceShellPage({ serviceId }: { serviceId: string }) {
  const { service, loading } = useServer(serviceId, { poll: false });
  const { t } = useTranslations();
  const command = service?.sshAddress ? `ssh ${service.sshAddress}` : "";
  const eligible = Boolean(service?.sshAddress);
  const unavailable = !loading && !eligible;

  return (
    <div className="space-y-6">
      <div className="space-y-1">
        <h2 className="text-xl font-semibold">{t("services.shellTitle")}</h2>
        <p className="text-muted-foreground text-sm">
          {t("services.shellDescription")}
        </p>
      </div>

      {loading && !service ? <ServiceShellCardsSkeleton /> : null}

      {eligible ? (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Terminal className="size-4" />
              {t("services.shellWebTitle")}
            </CardTitle>
            <CardDescription>
              {t("services.shellWebDescription")}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <WebShellPanel serviceId={serviceId} />
            <p className="text-muted-foreground text-sm">
              {t("services.shellSshFallback")}
            </p>
          </CardContent>
        </Card>
      ) : null}

      {loading && !service ? null : (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <SquareTerminal className="size-4" />
              {unavailable
                ? t("services.shellUnavailableTitle")
                : t("services.shellConnectionTitle")}
            </CardTitle>
            <CardDescription>
              {unavailable
                ? t("services.shellUnavailableBody")
                : t("services.shellConnectionDescription")}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {command ? (
              // Gate the copy-ready `ssh` command behind key registration
              // (w2/m66): with no key it fails, so show the add-key CTA instead.
              // The in-browser Web Shell card above is the zero-setup second door
              // (session-auth, no key), so this CTA needs no secondary action.
              <RequiresSshKey
                surface="service-shell"
                fallback={
                  <AddSshKeyCta
                    surface="service-shell"
                    returnTo={`/services/${serviceId}/shell`}
                  />
                }
              >
                <ConnectionField
                  label={t("services.shellCommand")}
                  value={command}
                  copiedText={t("services.sshCopied")}
                  copyErrorText={t("services.sshCopyError")}
                />
                <p className="text-muted-foreground text-sm">
                  {t("services.shellSessionLifecycle")}
                </p>
              </RequiresSshKey>
            ) : null}

            <Button asChild variant="outline" size="sm">
              <Link to="/settings" hash="ssh-public-keys">
                <KeyRound />
                {t("services.shellManageKeys")}
              </Link>
            </Button>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
