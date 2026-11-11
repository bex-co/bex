import { useMemo, useRef, useState } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  EditableFieldRow,
  EditableFieldRowPreview,
} from "@/features/services/components/editable-field-row";
import {
  keyValueClearPhrase,
  MAXMEMORY_POLICIES,
  persistenceKeepsData,
  RECOMMENDED_MAXMEMORY_POLICY,
} from "@/features/keyvalue/lib/labels";
import { useSetKeyValueMaxmemoryPolicy } from "@/features/keyvalue/hooks/use-set-key-value-maxmemory-policy";
import { useSetKeyValuePersistenceMode } from "@/features/keyvalue/hooks/use-set-key-value-persistence-mode";

/**
 * The Key Value detail's Maxmemory Policy section: change the eviction policy
 * after create (w7/007). The policy is already updatable on every programmatic
 * surface (REST PATCH / GraphQL setKeyValueMaxmemoryPolicy / MCP, w7/m45); this
 * closes the dashboard gap so it isn't a create-only field. Edit-in-place with
 * the shared select row (w5/m55): a disabled select + pencil that swaps for
 * Cancel / "Save changes", matching the services Settings page and Render.
 *
 * The policy is a valkey-server flag, so saving it replaces the store's only
 * pod. Save confirms that restart first, and on a running Persistence Off
 * store, which reloads nothing, asks for the typed phrase the persistence and
 * plan changes use for the same loss (w4/m316).
 */
export function KeyValueMaxmemoryPolicySection({
  id,
  name = id,
  suspended = false,
  onChanged,
}: {
  id: string;
  name?: string;
  /** A suspended store has no process to restart, so it is not asked. */
  suspended?: boolean;
  /** Fired after a successful save, which restarts the store (w4/m137). */
  onChanged?: () => void;
}) {
  const { t } = useTranslations();
  const { policy, loading, saving, save } = useSetKeyValueMaxmemoryPolicy(
    id,
    onChanged,
  );
  const { mode: persistenceMode } = useSetKeyValuePersistenceMode(id);
  const losesData = !persistenceKeepsData(persistenceMode);
  const [pending, setPending] = useState<string | null>(null);
  const [editRevision, setEditRevision] = useState(0);
  // save() resolves after the mutation's own loading flag clears (it settles
  // the page's reads first), so the dialog stays busy until it returns.
  const [submitting, setSubmitting] = useState(false);
  const confirming = useRef(false);
  // Stable identity so EditableFieldRow's focus effect doesn't re-run each render.
  const options = useMemo(
    () =>
      MAXMEMORY_POLICIES.map((p) => ({
        value: p,
        label:
          p === RECOMMENDED_MAXMEMORY_POLICY
            ? `${p} ${t("keyvalue.fieldMaxmemoryRecommended")}`
            : p,
      })),
    [t],
  );

  async function requestSave(next: string) {
    if (suspended) return save(next);
    setPending(next);
    return false;
  }

  async function confirmSave() {
    if (pending === null || confirming.current) return;
    confirming.current = true;
    setSubmitting(true);
    try {
      if (await save(pending)) {
        setPending(null);
        setEditRevision((revision) => revision + 1);
      }
    } finally {
      confirming.current = false;
      setSubmitting(false);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("keyvalue.maxmemoryTitle")}</CardTitle>
        <CardDescription>{t("keyvalue.maxmemoryDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        {loading && !policy ? (
          // The field's own anatomy, so the row keeps its bounds (w4/299).
          <EditableFieldRowPreview
            label={t("keyvalue.maxmemoryLabel")}
            hint={t("keyvalue.fieldMaxmemoryPolicyHint")}
            select={{ narrowLines: 2 }}
          />
        ) : (
          <EditableFieldRow
            key={`${id}:${editRevision}`}
            label={t("keyvalue.maxmemoryLabel")}
            hint={t("keyvalue.fieldMaxmemoryPolicyHint")}
            value={policy}
            editLabel={t("keyvalue.maxmemoryEdit")}
            busy={saving}
            options={options}
            onSave={requestSave}
          />
        )}
      </CardContent>
      <ConfirmDialog
        key={`${id}:${name}:${pending}`}
        open={pending !== null}
        onOpenChange={(open) => {
          if (!open && !saving && !submitting) setPending(null);
        }}
        title={t("keyvalue.maxmemoryConfirmTitle", { policy: pending ?? "" })}
        description={[
          t("keyvalue.maxmemoryConfirmBody"),
          losesData
            ? t(
                persistenceMode === "off"
                  ? "keyvalue.restartConfirmDataLoss"
                  : "keyvalue.restartConfirmDataLossUnknown",
                { name },
              )
            : "",
        ]
          .filter(Boolean)
          .join(" ")}
        phrase={losesData ? keyValueClearPhrase(name) : undefined}
        confirmLabel={t(
          losesData
            ? "keyvalue.maxmemoryConfirmDataLossAction"
            : "keyvalue.maxmemoryConfirmAction",
        )}
        destructive={losesData}
        pending={saving || submitting}
        closeOnConfirm={false}
        onConfirm={() => void confirmSave()}
      />
    </Card>
  );
}
