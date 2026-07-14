import { useState, type ReactNode } from "react";
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
  initialChecked: string[];
  busy: boolean;
  emptyLabel: string;
  onSave: (ids: string[]) => Promise<boolean>;
  onClose: () => void;
}

/**
 * One resource-kind tab of the environment "Manage resources" dialog: a
 * checkbox list of candidates, pre-checked for those already assigned,
 * full-replacing on save — the shape `AssignServicesForm` used before w6/m20
 * generalized it to also cover databases and key-value instances. Local
 * `checked` state seeds fresh each mount (Radix unmounts inactive
 * `TabsContent`/closed `Dialog` children), so no sync effect is needed.
 */
export function ResourceChecklist({
  items,
  initialChecked,
  busy,
  emptyLabel,
  onSave,
  onClose,
}: ResourceChecklistProps) {
  const { t } = useTranslations();
  const [checked, setChecked] = useState<Set<string>>(
    () => new Set(initialChecked),
  );

  function toggle(id: string, next: boolean) {
    setChecked((prev) => {
      const nextSet = new Set(prev);
      if (next) nextSet.add(id);
      else nextSet.delete(id);
      return nextSet;
    });
  }

  async function handleSave() {
    const ok = await onSave([...checked]);
    if (ok) onClose();
  }

  return (
    <>
      {items.length === 0 ? (
        <p className="py-4 text-sm text-muted-foreground">{emptyLabel}</p>
      ) : (
        <ScrollArea className="max-h-72 pr-3">
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
                  onCheckedChange={(v) => toggle(item.id, v === true)}
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
