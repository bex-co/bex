import { ExternalLink } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { Badge } from "@/common/components/ui/badge";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * The Platform Subdomain section, below Custom Domains on the Settings tab. This
 * is bex's counterpart to Render's "Render Subdomain" toggle — but bex always
 * keeps the platform `.onbex.co` subdomain reachable (there's no opt-out), so it's
 * a read-only display of the service's URL, not a toggle. Sourced from the same
 * `server(id)` the settings page already loads.
 */
export function PlatformSubdomainSection({ url }: { url: string | null }) {
  const { t } = useTranslations();

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
          {url ? (
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
          )}
          <Badge variant="success">
            {t("services.platformSubdomainEnabled")}
          </Badge>
        </div>
      </CardContent>
    </Card>
  );
}
