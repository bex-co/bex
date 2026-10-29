import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { IPAllowListEditor } from "@/common/components/ip-allow-list-editor";
import { ipAllowListEntryKey } from "@/common/lib/ip-allow-list";
import { useRef, useState } from "react";
import { ProtectedConfirmationDialog } from "@/common/components/protected-confirmation-dialog";
import type { IPAllowListEntryDraft } from "@/common/lib/ip-allow-list";
import { useTranslations } from "@/common/hooks/use-translations";
import { KeyValueNetworkingPreview } from "@/common/components/datastore-overview-previews";
import {
  useKeyValueNetworking,
  type RestartRefusal,
} from "@/features/keyvalue/hooks/use-key-value-networking";

/**
 * The Key Value detail's Networking section: the external-endpoint IP
 * allowlist (editable CIDR list) — Render's Networking control, mirroring the
 * allowlist section of databases' AccessControlPanel. Saving rules enables
 * external access; saving an empty list disables it. Only a store's first
 * enable restarts it (w4/m233): the hint says so before Save, and when bex-api
 * refuses that save because the restart would empty a Persistence Off store,
 * the panel asks for the phrase the refusal names and saves once more with it.
 */
export function KeyValueNetworkingPanel({ id }: { id: string }) {
  const { t } = useTranslations();
  const networking = useKeyValueNetworking(id);
  const [pending, setPending] = useState<{
    entries: IPAllowListEntryDraft[];
    refusal: RestartRefusal;
  } | null>(null);
  const confirming = useRef(false);

  async function requestSave(entries: IPAllowListEntryDraft[]) {
    const result = await networking.saveAllowList(entries);
    if (typeof result !== "object") return result;
    setPending({ entries, refusal: result });
    return false;
  }

  async function confirmSave(confirmation: string) {
    if (!pending || confirming.current) return;
    confirming.current = true;
    try {
      const result = await networking.saveAllowList(
        pending.entries,
        confirmation,
      );
      if (result === true) setPending(null);
      else if (typeof result === "object")
        setPending({ ...pending, refusal: result });
    } finally {
      confirming.current = false;
    }
  }

  // The page's Key Value read can settle before this card's own allowlist
  // read. Until then the status line and empty copy are unknown, so keep the
  // route preview rather than drawing a shorter card that regrows (w4/303).
  if (networking.loading && networking.isPublic === undefined) {
    return <KeyValueNetworkingPreview />;
  }

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
          onSave={requestSave}
          allowUnchangedSave={
            networking.isPublic != null &&
            networking.isPublic !== Boolean(networking.allowList.length)
          }
          labels={{
            hint: t(
              networking.externalAccessRestarts
                ? "keyvalue.networkingHintRestart"
                : "keyvalue.networkingHint",
            ),
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
      {/* Mounted per refusal, so a reopened dialog never starts with a phrase
          typed for an earlier one. */}
      {pending ? (
        <ProtectedConfirmationDialog
          key={pending.refusal.confirm}
          open
          resourceName={pending.refusal.name}
          requiredConfirmation={pending.refusal.confirm}
          title={t("keyvalue.networkingRestartConfirmTitle")}
          description={t("keyvalue.networkingRestartConfirmDescription", {
            name: pending.refusal.name,
          })}
          actionLabel={t("keyvalue.networkingRestartConfirmAction")}
          busy={networking.savingAllowList}
          onOpenChange={(open) => {
            if (!open && !networking.savingAllowList) setPending(null);
          }}
          onConfirm={confirmSave}
        />
      ) : null}
    </Card>
  );
}
