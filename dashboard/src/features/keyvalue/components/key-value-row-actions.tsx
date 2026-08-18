import { useState } from "react";
import { MoreHorizontal, Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { useTranslations } from "@/common/hooks/use-translations";
import { DeleteKeyValueDialog } from "@/features/keyvalue/components/key-value-confirm-dialogs";
import { useDeleteKeyValue } from "@/features/keyvalue/hooks/use-delete-key-value";
import type { KeyValueView } from "@/features/keyvalue/types";
import { MoveToProjectMenu } from "@/features/projects/components/move-to-project-menu";
import { ProtectedConfirmationDialog } from "@/common/components/protected-confirmation-dialog";
import { PermissionMenuItem } from "@/features/capabilities/components/permission-menu-item";
import {
  useCapabilities,
  type Capabilities,
} from "@/features/capabilities/hooks/use-capabilities";

export interface KeyValueRowActionsProps {
  keyValue: KeyValueView;
  /** Called after a successful delete (refetch the list / leave the detail page). */
  onDeleted: (id: string) => void;
}

/**
 * The per-store actions menu. Delete is the only lifecycle verb this menu
 * serves — suspend/resume live on the detail page's own action bar (matching
 * Render's KV detail capture, docs/render-artifacts/key-value.md), not this
 * compact row menu. Destructive and irreversible (cascades the Valkey
 * StatefulSet + PVC), so it uses the same exact typed sudo confirmation as
 * Render's Key Value detail page.
 */
export function KeyValueRowActions(props: KeyValueRowActionsProps) {
  const capabilities = useCapabilities();
  return (
    <KeyValueRowActionsWithCapabilities
      {...props}
      capabilities={capabilities}
    />
  );
}

export function KeyValueRowActionsWithCapabilities({
  keyValue,
  onDeleted,
  capabilities,
}: KeyValueRowActionsProps & {
  capabilities: Pick<Capabilities, "canCreate" | "loaded">;
}) {
  const { t } = useTranslations();
  const { canCreate, loaded: capabilitiesLoaded } = capabilities;
  const createDenied = capabilitiesLoaded && !canCreate;
  const createReason = createDenied
    ? t("capabilities.reasonCanCreate")
    : undefined;
  const { remove, deleting } = useDeleteKeyValue();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [protectedConfirmation, setProtectedConfirmation] = useState<
    string | null
  >(null);

  const busy = deleting === keyValue.id;

  async function handleDelete(confirmation?: string) {
    if (createDenied) return;
    const result = confirmation
      ? await remove(keyValue.id, keyValue.name, confirmation)
      : await remove(keyValue.id, keyValue.name);
    if (result.status === "confirmation_required") {
      setConfirmOpen(false);
      setProtectedConfirmation(result.confirmation);
    } else if (result.status === "success") {
      setConfirmOpen(false);
      setProtectedConfirmation(null);
      onDeleted(keyValue.id);
    }
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="icon-sm"
            disabled={busy}
            aria-label={t("keyvalue.actionsMenu")}
          >
            {busy ? <Loader2 className="animate-spin" /> : <MoreHorizontal />}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <PermissionMenuItem
            permissionReason={createReason}
            variant="destructive"
            onSelect={() => {
              if (!createDenied) setConfirmOpen(true);
            }}
          >
            {t("keyvalue.actionDelete")}
          </PermissionMenuItem>
          <MoveToProjectMenu
            kind="keyvalue"
            resourceId={keyValue.id}
            resourceName={keyValue.name}
            disabled={busy}
          />
        </DropdownMenuContent>
      </DropdownMenu>

      <DeleteKeyValueDialog
        keyValue={keyValue}
        open={confirmOpen && !createDenied}
        onOpenChange={(open) => {
          if (!createDenied) setConfirmOpen(open);
        }}
        busy={busy}
        onConfirm={handleDelete}
      />

      <ProtectedConfirmationDialog
        key={protectedConfirmation ? `open:${protectedConfirmation}` : "closed"}
        open={protectedConfirmation !== null && !createDenied}
        resourceName={keyValue.name}
        requiredConfirmation={protectedConfirmation ?? ""}
        actionLabel={t("keyvalue.deleteConfirm")}
        busy={busy}
        onOpenChange={(open) => !open && setProtectedConfirmation(null)}
        onConfirm={handleDelete}
      />
    </>
  );
}
