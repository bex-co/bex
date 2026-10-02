import { useMemo, useRef, useState } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";
import { PERSISTENCE_MODES } from "@/features/keyvalue/lib/labels";
import { useSetKeyValuePersistenceMode } from "@/features/keyvalue/hooks/use-set-key-value-persistence-mode";
import type { en } from "@/i18n";

const PERSISTENCE_LABEL_KEYS: Record<
  (typeof PERSISTENCE_MODES)[number],
  keyof typeof en
> = {
  "journal-snapshot": "keyvalue.persistenceJournalSnapshot",
  snapshot: "keyvalue.persistenceSnapshot",
  off: "keyvalue.persistenceOff",
};

/**
 * Key Value detail Persistence Mode section — mirrors Maxmemory Policy so the
 * post-create updatable API field is also editable in the dashboard (w4/066).
 */
export function KeyValuePersistenceModeSection({
  id,
  name = id,
  onChanged,
}: {
  id: string;
  name?: string;
  /** Fired after a successful save, which restarts the store (w4/m137). */
  onChanged?: () => void;
}) {
  const { t } = useTranslations();
  const { mode, loading, saving, save } = useSetKeyValuePersistenceMode(
    id,
    onChanged,
  );
  const [pending, setPending] = useState<{
    id: string;
    from: string;
    to: string;
  } | null>(null);
  const [editRevision, setEditRevision] = useState(0);
  const confirming = useRef(false);
  const confirmationCurrent = pending?.id === id && pending.from === mode;

  async function requestSave(next: string) {
    if (!mode) return false;
    if (next !== mode && (mode === "off" || next === "off")) {
      setPending({ id, from: mode, to: next });
      return false;
    }
    return save(next);
  }

  async function confirmSave() {
    if (!pending || !confirmationCurrent || confirming.current) return;
    confirming.current = true;
    try {
      if (await save(pending.to)) {
        setPending(null);
        setEditRevision((revision) => revision + 1);
      }
    } finally {
      confirming.current = false;
    }
  }

  const options = useMemo(
    () =>
      PERSISTENCE_MODES.map((value) => ({
        value,
        label: t(PERSISTENCE_LABEL_KEYS[value]),
      })),
    [t],
  );

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("keyvalue.persistenceTitle")}</CardTitle>
        <CardDescription>
          {t("keyvalue.persistenceDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {loading && !mode ? (
          <Skeleton className="h-9 w-full" />
        ) : (
          <EditableFieldRow
            key={`${id}:${editRevision}`}
            label={t("keyvalue.persistenceLabel")}
            hint={t("keyvalue.fieldPersistenceModeHint")}
            value={mode}
            editLabel={t("keyvalue.persistenceEdit")}
            busy={saving}
            disabled={!mode}
            options={options}
            onSave={requestSave}
          />
        )}
      </CardContent>
      <ConfirmDialog
        key={`${id}:${name}:${mode}:${pending?.from}:${pending?.to}`}
        open={pending !== null && confirmationCurrent}
        onOpenChange={(open) => {
          if (!open && !saving) setPending(null);
        }}
        title={t("keyvalue.persistenceDiscardTitle")}
        description={t("keyvalue.persistenceDiscardBody")}
        confirmLabel={t("keyvalue.persistenceDiscardConfirm")}
        phrase={`sudo discard key value ${name}`}
        pending={saving}
        closeOnConfirm={false}
        onConfirm={() => void confirmSave()}
      />
    </Card>
  );
}
