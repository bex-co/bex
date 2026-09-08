import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { ConnectionField } from "@/common/components/connection-field";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * Settings card listing the service's shared egress IPs (Render Connect panel
 * parity, w8/010). Values come from GraphQL `Service.outboundIps` — always
 * `type: "shared"` on bex; an empty list is honest (local CAPD), not an error.
 */
export function ServiceOutboundIpsPanel({
  ips,
}: {
  ips: readonly string[] | null | undefined;
}) {
  const { t } = useTranslations();
  const list = (ips ?? []).filter((ip): ip is string => !!ip);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.outboundIpsTitle")}</CardTitle>
        <CardDescription>
          {t("services.outboundIpsDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {list.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            {t("services.outboundIpsEmpty")}
          </p>
        ) : (
          list.map((ip) => (
            <ConnectionField
              key={ip}
              label={t("services.outboundIpsCopy", { ip })}
              value={ip}
              copiedText={t("services.outboundIpsCopied")}
              copyErrorText={t("services.outboundIpsCopyError")}
            />
          ))
        )}
      </CardContent>
    </Card>
  );
}
