import { type ReactNode } from "react";
import { Loader2 } from "lucide-react";
import { DialogFooter } from "@/common/components/ui/dialog";
import { Button } from "@/common/components/ui/button";
import { Checkbox } from "@/common/components/ui/checkbox";
import { Label } from "@/common/components/ui/label";
import { ScrollArea } from "@/common/components/ui/scroll-area";
import { useTranslations } from "@/common/hooks/use-translations";

export interface ResourceChecklistItem {
  id: string;
  name: string;
  badge: ReactNode;
}

export interface ResourceChecklistProps {
  items: ResourceChecklistItem[];
  /** The tab's draft selection. Owned by the parent — see the note below. */
  checked: Set<string>;
  onToggle: (id: string, next: boolean) => void;
  busy: boolean;
  emptyLabel: string;
  onSave: (ids: string[]) => Promise<boolean>;
  onClose: () => void;
}

/**
 * One resource-kind tab of the environment "Manage resources" dialog: a
 * checkbox list of candidates, pre-checked for those already assigned,
 * full-replacing on save — the shape `AssignServicesForm` used before w6/m20
 * generalized it to also cover databases and key-value instances.
 *
 * Controlled, deliberately. This owned `checked` locally and seeded it on
 * mount, which was correct for the dialog (Radix unmounts a closed Dialog's
 * children, so reopening SHOULD reseed) but wrong for the tabs: Radix unmounts
 * inactive `TabsContent` too, so visiting another kind and coming back
 * destroyed the draft and reseeded from unchanged server membership — the
 * user's unchecked box silently checked itself again (w4/133). The drafts now
 * live in `ManageResourcesForm`, which stays mounted across tab changes and is
 * itself remounted per dialog open, so both behaviours come out right.
 */
export function ResourceChecklist({
  items,
  checked,
  onToggle,
  busy,
  emptyLabel,
  onSave,
  onClose,
}: ResourceChecklistProps) {
  const { t } = useTranslations();

  async function handleSave() {
    const ok = await onSave([...checked]);
    if (ok) onClose();
  }

  return (
    <>
      {items.length === 0 ? (
        <p className="py-4 text-sm text-muted-foreground">{emptyLabel}</p>
      ) : (
        // Height constraint on the VIEWPORT, not the root — a root max-h
        // constrains nothing and the list overflows the dialog onto the
        // footer buttons (the w1/m49 webhook-modal bug).
        <ScrollArea className="pr-3" viewportClassName="max-h-72">
          <div className="space-y-1">
            {items.map((item) => (
              <Label
                key={item.id}
                htmlFor={`assign-${item.id}`}
                className="flex cursor-pointer items-center gap-3 rounded-md px-2 py-2 hover:bg-muted/50"
              >
                <Checkbox
                  id={`assign-${item.id}`}
                  checked={checked.has(item.id)}
                  onCheckedChange={(v) => onToggle(item.id, v === true)}
                  disabled={busy}
                />
                <span className="flex-1 font-medium">{item.name}</span>
                {item.badge}
              </Label>
            ))}
          </div>
        </ScrollArea>
      )}

      <DialogFooter>
        <Button variant="outline" onClick={onClose} disabled={busy}>
          {t("environments.cancel")}
        </Button>
        <Button onClick={() => void handleSave()} disabled={busy}>
          {busy ? <Loader2 className="animate-spin" /> : null}
          {t("environments.manageSubmit")}
        </Button>
      </DialogFooter>
    </>
  );
}
