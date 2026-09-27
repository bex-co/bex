import { ExternalLink } from "lucide-react";
import { Switch } from "@/common/components/ui/switch";
import { Label } from "@/common/components/ui/label";
import { useTranslations } from "@/common/hooks/use-translations";
import { safeHttpHref } from "@/common/lib/external-url";
import { useSubdomainPolicy } from "@/features/services/hooks/use-subdomain-policy";
import type { CustomDomainView } from "@/features/services/types";

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
  domains,
  withHeading = true,
}: {
  serviceId: string;
  url: string | null;
  renderSubdomainPolicy: string | null | undefined;
  /**
   * The service's custom domains, when the caller has them. The server keeps
   * the platform subdomain on until one of them is verified (a pending domain
   * never serves), so the switch explains that up front instead of a refusal
   * toast after the click (w4/142). Absent => no pre-check.
   */
  domains?: Pick<CustomDomainView, "name" | "ownershipVerified">[];
  withHeading?: boolean;
}) {
  const { t } = useTranslations();
  const { setSubdomainPolicy, busy } = useSubdomainPolicy();
  const enabled = (renderSubdomainPolicy ?? "enabled") === "enabled";
  const pending = (domains ?? []).filter((d) => !d.ownershipVerified);
  const cannotDisable =
    enabled && !!domains && !domains.some((d) => d.ownershipVerified);
  const disableHint = !cannotDisable
    ? null
    : pending.length > 0
      ? t("services.platformSubdomainNeedsVerifiedDomainPending", {
          names: pending.map((d) => d.name).join(", "),
        })
      : t("services.platformSubdomainNeedsVerifiedDomain");
  // Never place a non-http(s) scheme into href (codex-security target #4).
  const safeUrl = safeHttpHref(url);

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
          safeUrl ? (
            <a
              href={safeUrl}
              target="_blank"
              rel="noreferrer noopener"
              className="inline-flex items-center gap-1 font-medium break-all hover:underline"
            >
              {url}
              <ExternalLink className="text-muted-foreground size-3" />
            </a>
          ) : url ? (
            <span className="text-muted-foreground text-sm break-all">
              {url}
            </span>
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
            disabled={busy || cannotDisable}
            onCheckedChange={(checked) =>
              void setSubdomainPolicy(
                serviceId,
                checked ? "enabled" : "disabled",
              )
            }
            aria-label={t("services.platformSubdomainToggleLabel")}
            aria-describedby={
              disableHint ? "platform-subdomain-hint" : undefined
            }
          />
        </div>
      </div>
      {disableHint && (
        <p
          id="platform-subdomain-hint"
          className="text-muted-foreground text-sm"
        >
          {disableHint}
        </p>
      )}
    </div>
  );
}
