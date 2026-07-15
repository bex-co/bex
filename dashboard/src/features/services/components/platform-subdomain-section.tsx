import { ExternalLink } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { Switch } from "@/common/components/ui/switch";
import { Label } from "@/common/components/ui/label";
import { useTranslations } from "@/common/hooks/use-translations";
import { useSubdomainPolicy } from "@/features/services/hooks/use-subdomain-policy";

/**
 * The Platform Subdomain section, below Custom Domains on the Settings tab. This
 * is bex's counterpart to Render's "Render Subdomain" toggle (w7/m31,
 * renderSubdomainPolicy: enabled | disabled): lets a tenant with custom domains
 * opt the platform `.onbex.co` subdomain out so the service stops answering there.
 * Sourced from the same `server(id)` the settings page already loads.
 */
export function PlatformSubdomainSection({
  serviceId,
  url,
  renderSubdomainPolicy,
}: {
  serviceId: string;
  url: string | null;
  /** Render's renderSubdomainPolicy field: "enabled" | "disabled". */
  renderSubdomainPolicy: string | null | undefined;
}) {
  const { t } = useTranslations();
  const { setSubdomainPolicy, busy } = useSubdomainPolicy();
  const enabled = (renderSubdomainPolicy ?? "enabled") === "enabled";

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.platformSubdomainTitle")}</CardTitle>
        <CardDescription>
          {t("services.platformSubdomainDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex items-center justify-between gap-4">
          {enabled ? (
            url ? (
              <a
                href={url}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-1 font-medium break-all hover:underline"
              >
                {url}
                <ExternalLink className="text-muted-foreground size-3" />
              </a>
            ) : (
              <span className="text-muted-foreground text-sm">
                {t("services.platformSubdomainPending")}
              </span>
            )
          ) : (
            <span className="text-muted-foreground text-sm">
              {t("services.platformSubdomainDisabledNote")}
            </span>
          )}
          <div className="flex items-center gap-2 shrink-0">
            <Label htmlFor="platform-subdomain-switch" className="text-sm">
              {enabled
                ? t("services.platformSubdomainEnabled")
                : t("services.platformSubdomainDisabled")}
            </Label>
            <Switch
              id="platform-subdomain-switch"
              checked={enabled}
              disabled={busy}
              onCheckedChange={(checked) =>
                void setSubdomainPolicy(
                  serviceId,
                  checked ? "enabled" : "disabled",
                )
              }
              aria-label={t("services.platformSubdomainToggleLabel")}
            />
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
