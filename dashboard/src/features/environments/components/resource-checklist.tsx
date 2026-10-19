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
  /** Where the resource is now, when that is another environment (w4/m232). */
  location?: string;
  /** What leaving that environment removes: Protected, Isolated, … */
  markers?: string[];
  /** Set when this viewer cannot move it here; the row says why. */
  disabledReason?: string;
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
  /** Before Save: one line per environment the selection moves resources out of. */
  moveSummary?: string[];
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
  moveSummary = [],
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
            {items.map((item) => {
              const detail = Boolean(item.location || item.disabledReason);
              return (
                <Label
                  key={item.id}
                  htmlFor={`assign-${item.id}`}
                  className="flex cursor-pointer items-center gap-3 rounded-md px-2 py-2 hover:bg-muted/50"
                >
                  {/* Named by the resource alone, with where it is and why it
                      is locked as its description, not one run-on label. */}
                  <Checkbox
                    id={`assign-${item.id}`}
                    aria-labelledby={`assign-${item.id}-name`}
                    aria-describedby={
                      detail ? `assign-${item.id}-detail` : undefined
                    }
                    checked={checked.has(item.id)}
                    onCheckedChange={(v) => onToggle(item.id, v === true)}
                    disabled={busy || Boolean(item.disabledReason)}
                  />
                  <span className="min-w-0 flex-1">
                    <span
                      id={`assign-${item.id}-name`}
                      className="block font-medium"
                    >
                      {item.name}
                    </span>
                    {detail && (
                      <span
                        id={`assign-${item.id}-detail`}
                        className="block text-xs font-normal text-muted-foreground"
                      >
                        {item.location && (
                          <span className="block">
                            {item.location}
                            {item.markers?.map((marker) => (
                              <span
                                key={marker}
                                className="ml-1.5 inline-block rounded bg-amber-500/15 px-1 py-0.5 whitespace-nowrap text-amber-700 dark:text-amber-400"
                              >
                                {marker}
                              </span>
                            ))}
                          </span>
                        )}
                        {item.disabledReason && (
                          <span className="block">{item.disabledReason}</span>
                        )}
                      </span>
                    )}
                  </span>
                  {item.badge}
                </Label>
              );
            })}
          </div>
        </ScrollArea>
      )}

      {/* Mounted while empty: a live region announces only changes to one
          that already exists. */}
      <div role="status">
        {moveSummary.length > 0 && (
          <ul className="space-y-0.5 rounded-md border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-sm">
            {moveSummary.map((line) => (
              <li key={line}>{line}</li>
            ))}
          </ul>
        )}
      </div>
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
