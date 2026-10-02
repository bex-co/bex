import { AlertTriangle } from "lucide-react";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * Warns that an inbound IP allowlist cannot see real client addresses on
 * Cloudflare-proxied custom domains (w1/m171). bex-api names those hosts in
 * `ipAllowListProxiedDomains` on the service and the environment; the save is
 * accepted (m150 Decision 2 warns rather than refuses), so this is the only
 * place a tenant learns the allowlist matches Cloudflare's edge there.
 * Renders nothing when the list is empty or absent.
 */
export function IPAllowListProxiedNotice({
  domains,
}: {
  domains: ReadonlyArray<string | null> | null | undefined;
}) {
  const { t } = useTranslations();
  const hosts = (domains ?? []).filter(
    (host): host is string => host != null && host !== "",
  );
  if (hosts.length === 0) return null;
  return (
    <Alert data-testid="ip-allow-list-proxied-notice">
      <AlertTriangle className="text-amber-500" />
      <AlertTitle className="line-clamp-none">
        {t("common.ipAllowListProxiedTitle", { count: hosts.length })}
      </AlertTitle>
      <AlertDescription>
        <p>{t("common.ipAllowListProxiedBody", { count: hosts.length })}</p>
        <ul className="list-disc pl-4 font-mono text-xs">
          {hosts.map((host) => (
            <li key={host}>{host}</li>
          ))}
        </ul>
        <p>{t("common.ipAllowListProxiedFix")}</p>
      </AlertDescription>
    </Alert>
  );
}
