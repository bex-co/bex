import { useTranslations } from "@/common/hooks/use-translations";
import { isValidDnsLabel } from "@/common/lib/utils/dns-label";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";
import { useRenameKeyValue } from "@/features/keyvalue/hooks/use-rename-key-value";
import type { KeyValueView } from "@/features/keyvalue/types";

export interface KeyValueNameRowProps {
  keyValue: KeyValueView;
  onRenamed: () => void;
}

/**
 * The Key Value store's editable display name, rendered with the shared
 * edit-in-place row (w5/m55) — a visibly disabled input + pencil that swaps for
 * Cancel / "Save changes", matching the services Settings page and Render's
 * datastore General section. The store ID and connection details never change
 * on rename, so the row leads the read-only Details card rather than owning one.
 */
export function KeyValueNameRow({ keyValue, onRenamed }: KeyValueNameRowProps) {
  const { t } = useTranslations();
  const { rename, busy } = useRenameKeyValue();

  return (
    <EditableFieldRow
      label={t("keyvalue.fieldName")}
      hint={t("keyvalue.nameDescription")}
      value={keyValue.name}
      editLabel={t("keyvalue.nameEdit")}
      busy={busy}
      validate={(draft) =>
        isValidDnsLabel(draft.trim()) ? null : t("keyvalue.nameInvalid")
      }
      onSave={async (value) => {
        const ok = await rename(keyValue.id, value);
        if (ok) onRenamed();
        return ok;
      }}
    />
  );
}
