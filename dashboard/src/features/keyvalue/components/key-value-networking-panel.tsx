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
import { useKeyValueNetworking } from "@/features/keyvalue/hooks/use-key-value-networking";

/**
 * The Key Value detail's Networking section: the external-endpoint IP
 * allowlist (editable CIDR list) — Render's Networking control, mirroring the
 * allowlist section of databases' AccessControlPanel. Saving rules enables
 * external access; saving an empty list disables it. Internal access is unchanged.
 */
export function KeyValueNetworkingPanel({ id }: { id: string }) {
  const { t } = useTranslations();
  const networking = useKeyValueNetworking(id);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("keyvalue.networkingTitle")}</CardTitle>
        <CardDescription>{t("keyvalue.networkingDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-2">
        {networking.isPublic === false ? (
          <p className="text-xs text-muted-foreground">
            {t("keyvalue.networkingInternalOnly")}
          </p>
        ) : null}
        {/* key remounts the editable draft from the server list whenever it
            changes (e.g. after a save), avoiding an effect-based state sync. */}
        <IPAllowListEditor
          key={ipAllowListEntryKey(networking.allowList)}
          entries={networking.allowList}
          saving={networking.savingAllowList}
          onSave={networking.saveAllowList}
          allowUnchangedSave={
            networking.isPublic != null &&
            networking.isPublic !== Boolean(networking.allowList.length)
          }
          labels={{
            hint: t("keyvalue.networkingHint"),
            empty: t(
              networking.isPublic && networking.allowList.length === 0
                ? "keyvalue.networkingPublicEmpty"
                : "keyvalue.networkingEmpty",
            ),
            descriptionPlaceholder: t("keyvalue.networkingEntryDescription"),
            cidr: t("keyvalue.networkingCIDR"),
            cidrRule: (number) => t("keyvalue.networkingCIDRRule", { number }),
            description: t("keyvalue.networkingDescriptionLabel"),
            descriptionRule: (number) =>
              t("keyvalue.networkingDescriptionRule", { number }),
            add: t("keyvalue.networkingAdd"),
            save: t("keyvalue.networkingSave"),
            invalid: t("keyvalue.networkingInvalid"),
            remove: (cidr) => t("keyvalue.networkingRemove", { cidr }),
            moveUp: (cidr) => t("keyvalue.networkingMoveUp", { cidr }),
            moveDown: (cidr) => t("keyvalue.networkingMoveDown", { cidr }),
          }}
        />
      </CardContent>
    </Card>
  );
}
