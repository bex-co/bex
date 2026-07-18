import { Link } from "@tanstack/react-router";
import { KeyRound, SquareTerminal } from "lucide-react";
import { ConnectionField } from "@/common/components/connection-field";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { useServer } from "@/features/services/hooks/use-server";

/**
 * Render names this sidebar destination Shell and hosts a browser terminal.
 * bex intentionally keeps execution out of the browser: this page exposes the
 * already-authorized running-instance OpenSSH path from ADR035 and never
 * handles a private key, terminal stream, or Kubernetes exec credential.
 */
export function ServiceShellPage({ serviceId }: { serviceId: string }) {
  const { service, loading } = useServer(serviceId);
  const { t } = useTranslations();
  const command = service?.sshAddress ? `ssh ${service.sshAddress}` : "";
  const unavailable = !loading && !command;

  return (
    <div className="space-y-6">
      <div className="space-y-1">
        <h2 className="text-xl font-semibold">{t("services.shellTitle")}</h2>
        <p className="text-muted-foreground text-sm">
          {t("services.shellDescription")}
        </p>
      </div>

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
          {loading && !service ? (
            <Skeleton className="h-16 w-full" />
          ) : command ? (
            <>
              <ConnectionField
                label={t("services.shellCommand")}
                value={command}
                copiedText={t("services.sshCopied")}
                copyErrorText={t("services.sshCopyError")}
              />
              <p className="text-muted-foreground text-sm">
                {t("services.shellSessionLifecycle")}
              </p>
            </>
          ) : null}

          <Button asChild variant="outline" size="sm">
            <Link to="/settings" hash="ssh-public-keys">
              <KeyRound />
              {t("services.shellManageKeys")}
            </Link>
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
