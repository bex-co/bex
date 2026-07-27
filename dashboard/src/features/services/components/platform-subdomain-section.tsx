import { ExternalLink } from "lucide-react";
import { Switch } from "@/common/components/ui/switch";
import { Label } from "@/common/components/ui/label";
import { useTranslations } from "@/common/hooks/use-translations";
import { useSubdomainPolicy } from "@/features/services/hooks/use-subdomain-policy";

/**
 * The platform-subdomain toggle row (URL link/note + on-off switch): bex's
 * counterpart to Render's "Render Subdomain" toggle (w7/m31,
 * renderSubdomainPolicy: enabled | disabled), letting a tenant with custom
 * domains opt the platform `.onbex.co` subdomain out. Embedded at the bottom of
 * the Custom Domains card (w5/m52); `withHeading` renders its own label +
 * description. Sourced from the same `server(id)` the settings page already loads.
 */
export function PlatformSubdomainRow({
  serviceId,
  url,
  renderSubdomainPolicy,
  withHeading = true,
}: {
  serviceId: string;
  url: string | null;
  renderSubdomainPolicy: string | null | undefined;
  withHeading?: boolean;
}) {
  const { t } = useTranslations();
  const { setSubdomainPolicy, busy } = useSubdomainPolicy();
  const enabled = (renderSubdomainPolicy ?? "enabled") === "enabled";

  return (
    <div className="space-y-3">
      {withHeading && (
        <div>
          <div className="text-sm font-medium">
            {t("services.platformSubdomainTitle")}
          </div>
          <p className="text-muted-foreground mt-1 text-sm">
            {t("services.platformSubdomainDescription")}
          </p>
        </div>
      )}
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
    </div>
  );
}
