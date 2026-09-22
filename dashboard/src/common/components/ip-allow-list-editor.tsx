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
  /** Accessible name for the "add" CIDR input (placeholder stays the example). */
  cidr: string;
  /** Accessible name for an existing-row CIDR; `{number}` is 1-based. */
  cidrRule: (number: number) => string;
  /** Accessible name for the "add" description input. */
  description: string;
  /** Accessible name for an existing-row description; `{number}` is 1-based. */
  descriptionRule: (number: number) => string;
  add: string;
  save: string;
  invalid: string;
  remove: (cidr: string) => string;
  moveUp: (cidr: string) => string;
  moveDown: (cidr: string) => string;
}

/**
 * One draft row: a stable editor-local identity plus the public entry shape.
 *
 * The identity is deliberately a WRAPPER rather than a field on the entry.
 * `IPAllowListEntryDraft` is the wire shape — the three save hooks forward
 * these objects straight into GraphQL variables, and `ipAllowListEntryKey`
 * compares them with JSON.stringify — so an id living on the entry would both
 * corrupt the dirty check and send an input field the schema does not declare.
 */
interface IPAllowListRow {
  /** Editor-local only. Never projected into onSave or the dirty comparison. */
  id: number;
  entry: IPAllowListEntryDraft;
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
  // Rows carry their own identity because React reconciles by key, and the key
  // used to be `${index}-${entry.cidrBlock}` — the editable value itself. One
  // Backspace in a CIDR field therefore changed the row's identity, so React
  // deleted the old row's DOM subtree and mounted a new one; the focused input
  // went with it and every keystroke after the first was dropped on the floor.
  // Description edits did not touch the key, which is exactly why they worked
  // and made the failure look field-specific (w4/135).
  //
  // Index alone cannot serve either: `move` reorders rows, and a render-time
  // generated value is new on every render, which is the same bug again.
  const [draft, setDraft] = useState<IPAllowListRow[]>(() =>
    entries.map((entry, index) => ({ id: index, entry })),
  );
  /**
   * Next free identity, derived from the rows themselves rather than held in a
   * ref — a ref cannot be read during render, and deriving it keeps `add` pure.
   * Monotonic against the CURRENT rows, so an id is never reused by a row added
   * after an earlier one was removed.
   */
  const nextRowID = (rows: IPAllowListRow[]) =>
    rows.reduce((max, row) => Math.max(max, row.id), -1) + 1;
  /** The public, ordered entries — what dirty, validation and Save all see. */
  const draftEntries = draft.map((row) => row.entry);
  const [cidr, setCIDR] = useState("");
  const [description, setDescription] = useState("");
  const [invalid, setInvalid] = useState(false);

  const dirty =
    ipAllowListEntryKey(draftEntries) !== ipAllowListEntryKey(entries);
  const draftInvalid =
    draftEntries.some((entry) => !isValidCIDR(entry.cidrBlock)) ||
    new Set(draftEntries.map((entry) => entry.cidrBlock.trim())).size !==
      draftEntries.length;

  function add() {
    const nextCIDR = cidr.trim();
    if (
      !isValidCIDR(nextCIDR) ||
      draftEntries.some((entry) => entry.cidrBlock === nextCIDR)
    ) {
      setInvalid(true);
      return;
    }
    setDraft([
      ...draft,
      {
        id: nextRowID(draft),
        entry: { cidrBlock: nextCIDR, description: description.trim() },
      },
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
      draft.map((row, rowIndex) =>
        rowIndex === index
          ? { ...row, entry: { ...row.entry, [field]: value } }
          : row,
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
          {draft.map(({ id, entry }, index) => (
            <div
              key={id}
              className="flex flex-wrap items-center gap-2 rounded-md border p-2"
            >
              <Input
                value={entry.cidrBlock}
                onChange={(event) =>
                  replace(index, "cidrBlock", event.target.value)
                }
                aria-invalid={!isValidCIDR(entry.cidrBlock)}
                aria-label={labels.cidrRule(index + 1)}
                className="max-w-xs font-mono"
              />
              <Input
                value={entry.description}
                onChange={(event) =>
                  replace(index, "description", event.target.value)
                }
                aria-label={labels.descriptionRule(index + 1)}
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
          aria-label={labels.cidr}
          aria-invalid={invalid}
          className="max-w-xs"
        />
        <Input
          value={description}
          onChange={(event) => setDescription(event.target.value)}
          onKeyDown={(event) =>
            event.key === "Enter" && (event.preventDefault(), add())
          }
          aria-label={labels.description}
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
          onClick={() => void onSave(draftEntries)}
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
