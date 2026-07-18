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
 * allowlist section of databases' AccessControlPanel. The allowlist only gates
 * the public SNI endpoint, so an internal-only store shows a note instead of
 * pretending the list has any effect.
 */
export function KeyValueNetworkingPanel({
  id,
  isPublic,
}: {
  id: string;
  isPublic: boolean;
}) {
  const { t } = useTranslations();
  const networking = useKeyValueNetworking(id);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("keyvalue.networkingTitle")}</CardTitle>
        <CardDescription>{t("keyvalue.networkingDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-2">
        {isPublic ? null : (
          <p className="text-xs text-muted-foreground">
            {t("keyvalue.networkingInternalOnly")}
          </p>
        )}
        {/* key remounts the editable draft from the server list whenever it
            changes (e.g. after a save), avoiding an effect-based state sync. */}
        <IPAllowListEditor
          key={ipAllowListEntryKey(networking.allowList)}
          entries={networking.allowList}
          saving={networking.savingAllowList}
          onSave={networking.saveAllowList}
          labels={{
            hint: t("keyvalue.networkingHint"),
            open: t("keyvalue.networkingOpen"),
            descriptionPlaceholder: t("keyvalue.networkingEntryDescription"),
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
