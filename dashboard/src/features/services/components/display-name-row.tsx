import { useTranslations } from "@/common/hooks/use-translations";
import { useDisplayName } from "@/features/services/hooks/use-display-name";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";

export interface DisplayNameRowProps {
  serviceId: string;
  /** Raw spec.displayName; empty means the immutable service name is displayed. */
  displayName: string | null | undefined;
  /** The immutable App name — the fallback value when no displayName is set
   *  (Render prefills its Name field with the service name, never the id;
   *  w5/m48/t004). Undefined while the service is still loading. */
  name?: string | null;
  onChanged?: () => void;
}

/**
 * Settings row for a service's mutable human label. The immutable service name
 * is shown as the fallback (the id only when even the name is unknown) and
 * never edited, so a rename cannot change routing or any Kubernetes identity
 * derived from the id.
 */
export function DisplayNameRow({
  serviceId,
  displayName,
  name,
  onChanged,
}: DisplayNameRowProps) {
  const { t } = useTranslations();
  const { setDisplayName, busy } = useDisplayName();
  const current = displayName?.trim() ?? "";
  const visible = current || name?.trim() || serviceId;

  return (
    <EditableFieldRow
      label={t("services.displayNameLabel")}
      hint={t("services.displayNameHint", { id: serviceId })}
      value={visible}
      editLabel={t("services.displayNameEdit")}
      busy={busy}
      // Clearing an already-empty label back to the immutable-id fallback is
      // not a change; anything else that differs from the visible value is.
      dirty={(value) => value !== visible && !(value === "" && current === "")}
      onSave={async (value) => {
        const ok = await setDisplayName(serviceId, value);
        if (ok) onChanged?.();
        return ok;
      }}
    />
  );
}
