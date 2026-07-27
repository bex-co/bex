import { useTranslations } from "@/common/hooks/use-translations";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";
import { useRenameDatabase } from "@/features/databases/hooks/use-rename-database";
import type { DatabaseDetailView } from "@/features/databases/types";

const NAME_PATTERN = /^[a-z0-9](?:[a-z0-9-]{0,28}[a-z0-9])?$/;

export interface DatabaseNameRowProps {
  database: DatabaseDetailView;
  onRenamed: () => void;
}

/**
 * The database's editable display name, rendered with the shared edit-in-place
 * row (w5/m55) — a visibly disabled input + pencil that swaps for Cancel /
 * "Save changes", matching the services Settings page and Render's Postgres
 * General section. The database ID and connection details never change on
 * rename, so the row leads the read-only Details card rather than owning a card.
 */
export function DatabaseNameRow({ database, onRenamed }: DatabaseNameRowProps) {
  const { t } = useTranslations();
  const { rename, busy } = useRenameDatabase();

  return (
    <EditableFieldRow
      label={t("databases.fieldName")}
      hint={t("databases.nameDescription")}
      value={database.name}
      editLabel={t("databases.nameEdit")}
      busy={busy}
      validate={(draft) =>
        NAME_PATTERN.test(draft.trim()) ? null : t("databases.nameInvalid")
      }
      onSave={async (value) => {
        const ok = await rename(database.id, value);
        if (ok) onRenamed();
        return ok;
      }}
    />
  );
}
