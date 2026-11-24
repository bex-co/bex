import type { ReactNode } from "react";
import { Copy, Pencil, Trash2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { PageHeader } from "@/common/components/page-frame";
import { useTranslations } from "@/common/hooks/use-translations";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";

/**
 * A project observability view's header (w3/m505), as Render's: the eyebrow
 * "Observability view" over the view's name, and the view's Edit view,
 * Duplicate and Delete buttons (w3/m507). The pending skeleton renders the
 * same header with a placeholder for the name and `ViewActionButtons`
 * without handlers, so the two share geometry.
 */
export function ObservabilityViewHeader({
  name,
  actions,
}: {
  name: ReactNode;
  actions?: ReactNode;
}) {
  const { t } = useTranslations();
  return (
    <div className="space-y-1">
      <p className="text-xs font-medium tracking-wider text-muted-foreground uppercase">
        {t("logs.observabilityViewEyebrow")}
      </p>
      <PageHeader title={name} actions={actions} />
    </div>
  );
}

/**
 * Render's three header buttons. Without handlers they are the pending
 * state's copies; a `reason` disables them and explains why in a
 * keyboard-reachable tooltip (docs/ADR018/surface-pins-and-corrections.md, Role-aware dashboard
 * controls).
 */
export function ViewActionButtons({
  onEdit,
  onDuplicate,
  onDelete,
  disabled = false,
  reason,
}: {
  onEdit?: () => void;
  onDuplicate?: () => void;
  onDelete?: () => void;
  disabled?: boolean;
  reason?: string;
}) {
  const { t } = useTranslations();
  const buttons = [
    { label: t("logs.editView"), icon: <Pencil />, onClick: onEdit },
    { label: t("logs.duplicateView"), icon: <Copy />, onClick: onDuplicate },
    {
      label: t("logs.deleteView"),
      icon: <Trash2 />,
      onClick: onDelete,
      iconOnly: true,
    },
  ];
  return (
    <>
      {buttons.map(({ label, icon, onClick, iconOnly }) => (
        <PermissionTooltip key={label} reason={reason}>
          <Button
            variant="outline"
            size={iconOnly ? "icon" : "sm"}
            className={iconOnly ? "size-8" : undefined}
            aria-label={iconOnly ? label : undefined}
            title={iconOnly ? label : undefined}
            disabled={disabled || reason !== undefined}
            onClick={onClick}
          >
            {icon}
            {iconOnly ? null : label}
          </Button>
        </PermissionTooltip>
      ))}
    </>
  );
}
