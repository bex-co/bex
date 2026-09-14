import { useMemo } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
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
export function KeyValuePersistenceModeSection({ id }: { id: string }) {
  const { t } = useTranslations();
  const { mode, loading, saving, save } = useSetKeyValuePersistenceMode(id);
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
            label={t("keyvalue.persistenceLabel")}
            hint={t("keyvalue.fieldPersistenceModeHint")}
            value={mode}
            editLabel={t("keyvalue.persistenceEdit")}
            busy={saving}
            options={options}
            onSave={save}
          />
        )}
      </CardContent>
    </Card>
  );
}
