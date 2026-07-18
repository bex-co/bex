import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { IPAllowListEditor } from "@/common/components/ip-allow-list-editor";
import { ipAllowListEntryKey } from "@/common/lib/ip-allow-list";
import { useTranslations } from "@/common/hooks/use-translations";
import { useServiceNetworking } from "@/features/services/hooks/use-service-networking";

/**
 * The service Settings Networking section: the inbound IP allowlist for
 * `web_service` and `static_site` (w7/m32, Render's ipAllowList). Mirrors
 * the keyvalue Networking panel in shape. The list is read from the service
 * detail already fetched by `useServer`; the panel only writes via mutation.
 */
export function ServiceNetworkingPanel({
  serviceId,
  currentAllowList,
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
  /** Called after a successful save so the parent can refetch if needed. */
  onSaved?: () => void;
}) {
  const { t } = useTranslations();
  const networking = useServiceNetworking();

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
        {/* key remounts the editable draft whenever the server list changes */}
        <IPAllowListEditor
          key={ipAllowListEntryKey(normalized)}
          entries={normalized}
          saving={networking.saving}
          onSave={async (entries) => {
            const ok = await networking.saveAllowList(serviceId, entries);
            if (ok) onSaved?.();
            return ok;
          }}
          labels={{
            hint: t("services.networkingHint"),
            open: t("services.networkingOpen"),
            descriptionPlaceholder: t("services.networkingEntryDescription"),
            add: t("services.networkingAdd"),
            save: t("services.networkingSave"),
            invalid: t("services.networkingInvalid"),
            remove: (cidr) => t("services.networkingRemove", { cidr }),
            moveUp: (cidr) => t("services.networkingMoveUp", { cidr }),
            moveDown: (cidr) => t("services.networkingMoveDown", { cidr }),
          }}
        />
      </CardContent>
    </Card>
  );
}
