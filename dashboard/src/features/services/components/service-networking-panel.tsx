import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { IPAllowListEditor } from "@/common/components/ip-allow-list-editor";
import { IPAllowListProxiedNotice } from "@/common/components/ip-allow-list-proxied-notice";
import { ipAllowListEntryKey } from "@/common/lib/ip-allow-list";
import { useTranslations } from "@/common/hooks/use-translations";
import { useServiceNetworking } from "@/features/services/hooks/use-service-networking";
import { useWorkspace } from "@/features/workspaces/context";
import { inboundIpRulesPlan } from "@/features/workspaces/types";

/**
 * The service Settings Networking section: the inbound IP rules for
 * `web_service` and `static_site` (w7/m32, Render's ipAllowList). Mirrors
 * the keyvalue Networking panel in shape. The list is read from the service
 * detail already fetched by `useServer`; the panel only writes via mutation.
 *
 * As on Render, a service nobody restricted has the allow-all rule, and a
 * service with NO rule refuses every public request (w8/m157).
 */
export function ServiceNetworkingPanel({
  serviceId,
  currentAllowList,
  proxiedDomains,
  onSaved,
}: {
  serviceId: string;
  /** Current description-preserving entries from the service detail query. */
  currentAllowList:
    | Array<{
        cidrBlock: string;
        description: string | null;
      } | null>
    | null
    | undefined;
  /**
   * Cloudflare-proxied custom domains on which this allowlist sees
   * Cloudflare's address instead of the client's (w1/m171).
   */
  proxiedDomains?: ReadonlyArray<string | null> | null;
  /** Called after a successful save so the parent can refetch if needed. */
  onSaved?: () => void;
}) {
  const { t } = useTranslations();
  const networking = useServiceNetworking();
  const { currentWorkspace } = useWorkspace();
  const belowScale = !inboundIpRulesPlan(currentWorkspace?.plan);

  // Not loaded yet is not "no rules": an empty list means the service is
  // blocked, so nothing is claimed until the list is known.
  const known = currentAllowList != null;
  const normalized = (currentAllowList ?? [])
    .filter((entry): entry is NonNullable<typeof entry> => entry != null)
    .map((entry) => ({
      cidrBlock: entry.cidrBlock,
      description: entry.description ?? "",
    }));

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.networkingTitle")}</CardTitle>
        <CardDescription>{t("services.networkingDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-2">
        <IPAllowListProxiedNotice domains={proxiedDomains} />
        {/* key remounts the editable draft whenever the server list changes */}
        {known ? (
          <IPAllowListEditor
            key={ipAllowListEntryKey(normalized)}
            entries={normalized}
            saving={networking.saving}
            clearOnly={
              belowScale
                ? {
                    hint: t("services.networkingScalePlanOnly"),
                    clearAll: t("services.networkingClearAll"),
                  }
                : undefined
            }
            emptyBlocks={{ open: t("services.networkingOpen") }}
            onSave={async (entries) => {
              const ok = await networking.saveAllowList(serviceId, entries);
              if (ok) onSaved?.();
              return ok;
            }}
            labels={{
              hint: t("services.networkingHint"),
              empty: t("services.networkingBlocked"),
              descriptionPlaceholder: t("services.networkingEntryDescription"),
              cidr: t("services.networkingCIDR"),
              cidrRule: (number) =>
                t("services.networkingCIDRRule", { number }),
              description: t("services.networkingDescriptionLabel"),
              descriptionRule: (number) =>
                t("services.networkingDescriptionRule", { number }),
              add: t("services.networkingAdd"),
              save: t("services.networkingSave"),
              invalid: t("services.networkingInvalid"),
              remove: (cidr) => t("services.networkingRemove", { cidr }),
              moveUp: (cidr) => t("services.networkingMoveUp", { cidr }),
              moveDown: (cidr) => t("services.networkingMoveDown", { cidr }),
            }}
          />
        ) : null}
      </CardContent>
    </Card>
  );
}
