import { useState } from "react";
import { ArrowDown, ArrowUp, Loader2, Plus, Trash2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { isValidCIDR } from "@/common/lib/cidr";
import {
  ipAllowListEntryKey,
  type IPAllowListEntryDraft,
} from "@/common/lib/ip-allow-list";

interface IPAllowListEditorLabels {
  hint: string;
  open: string;
  descriptionPlaceholder: string;
  add: string;
  save: string;
  invalid: string;
  remove: (cidr: string) => string;
  moveUp: (cidr: string) => string;
  moveDown: (cidr: string) => string;
}

/** Shared ordered CIDR + description editor for service and datastore ACLs. */
export function IPAllowListEditor({
  entries,
  labels,
  saving,
  onSave,
}: {
  entries: IPAllowListEntryDraft[];
  labels: IPAllowListEditorLabels;
  saving: boolean;
  onSave: (entries: IPAllowListEntryDraft[]) => Promise<boolean>;
}) {
  const [draft, setDraft] = useState(entries);
  const [cidr, setCIDR] = useState("");
  const [description, setDescription] = useState("");
  const [invalid, setInvalid] = useState(false);

  const dirty = ipAllowListEntryKey(draft) !== ipAllowListEntryKey(entries);
  const draftInvalid =
    draft.some((entry) => !isValidCIDR(entry.cidrBlock)) ||
    new Set(draft.map((entry) => entry.cidrBlock.trim())).size !== draft.length;

  function add() {
    const nextCIDR = cidr.trim();
    if (
      !isValidCIDR(nextCIDR) ||
      draft.some((entry) => entry.cidrBlock === nextCIDR)
    ) {
      setInvalid(true);
      return;
    }
    setDraft([
      ...draft,
      { cidrBlock: nextCIDR, description: description.trim() },
    ]);
    setCIDR("");
    setDescription("");
    setInvalid(false);
  }

  function replace(
    index: number,
    field: keyof IPAllowListEntryDraft,
    value: string,
  ) {
    setDraft(
      draft.map((entry, entryIndex) =>
        entryIndex === index ? { ...entry, [field]: value } : entry,
      ),
    );
  }

  function move(index: number, delta: -1 | 1) {
    const destination = index + delta;
    if (destination < 0 || destination >= draft.length) return;
    const next = [...draft];
    [next[index], next[destination]] = [next[destination], next[index]];
    setDraft(next);
  }

  return (
    <section className="space-y-2">
      <p className="text-xs text-muted-foreground">{labels.hint}</p>
      {draft.length === 0 ? (
        <span className="text-sm text-muted-foreground">{labels.open}</span>
      ) : (
        <div className="space-y-2">
          {draft.map((entry, index) => (
            <div
              key={`${index}-${entry.cidrBlock}`}
              className="flex flex-wrap items-center gap-2 rounded-md border p-2"
            >
              <Input
                value={entry.cidrBlock}
                onChange={(event) =>
                  replace(index, "cidrBlock", event.target.value)
                }
                aria-invalid={!isValidCIDR(entry.cidrBlock)}
                aria-label={`CIDR ${index + 1}`}
                className="max-w-xs font-mono"
              />
              <Input
                value={entry.description}
                onChange={(event) =>
                  replace(index, "description", event.target.value)
                }
                aria-label={`${labels.descriptionPlaceholder} ${index + 1}`}
                placeholder={labels.descriptionPlaceholder}
                className="max-w-xs"
              />
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label={labels.moveUp(entry.cidrBlock)}
                disabled={index === 0}
                onClick={() => move(index, -1)}
              >
                <ArrowUp />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label={labels.moveDown(entry.cidrBlock)}
                disabled={index === draft.length - 1}
                onClick={() => move(index, 1)}
              >
                <ArrowDown />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label={labels.remove(entry.cidrBlock)}
                onClick={() =>
                  setDraft(
                    draft.filter((_, entryIndex) => entryIndex !== index),
                  )
                }
              >
                <Trash2 />
              </Button>
            </div>
          ))}
        </div>
      )}
      <div className="flex flex-wrap gap-2">
        <Input
          value={cidr}
          onChange={(event) => {
            setCIDR(event.target.value);
            setInvalid(false);
          }}
          onKeyDown={(event) =>
            event.key === "Enter" && (event.preventDefault(), add())
          }
          placeholder="203.0.113.0/24"
          aria-invalid={invalid}
          className="max-w-xs"
        />
        <Input
          value={description}
          onChange={(event) => setDescription(event.target.value)}
          onKeyDown={(event) =>
            event.key === "Enter" && (event.preventDefault(), add())
          }
          placeholder={labels.descriptionPlaceholder}
          className="max-w-xs"
        />
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={add}
          disabled={!cidr.trim()}
        >
          <Plus />
          {labels.add}
        </Button>
        <Button
          type="button"
          size="sm"
          onClick={() => void onSave(draft)}
          disabled={!dirty || saving || draftInvalid}
        >
          {saving ? <Loader2 className="animate-spin" /> : null}
          {labels.save}
        </Button>
      </div>
      {invalid || draftInvalid ? (
        <p role="alert" className="text-xs text-destructive">
          {labels.invalid}
        </p>
      ) : null}
    </section>
  );
}
