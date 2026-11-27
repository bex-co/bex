import { useRef, useState, type ReactNode } from "react";
import { ArrowDown, ArrowUp, Loader2, Plus, Trash2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  TOUCH_TARGET_HEIGHT,
  TOUCH_TARGET_SIZE,
} from "@/common/lib/touch-target";
import { cn } from "@/common/lib/utils/utils";
import { isValidCIDR } from "@/common/lib/cidr";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import {
  ALLOW_ALL_ENTRIES,
  isAllowAllRule,
  ipAllowListEntryKey,
  type IPAllowListEntryDraft,
} from "@/common/lib/ip-allow-list";

interface IPAllowListEditorLabels {
  hint: string;
  empty: string;
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
  /**
   * When set, a Cancel button beside Save discards the unsaved draft (rows and
   * the add-row inputs) back to `entries`.
   */
  cancel?: string;
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

/**
 * One editable rule row of the editor (two inputs, three icon buttons) at the
 * ready row's geometry, for a list that has not loaded yet. `touch` is for a
 * row inside a `data-touch-region`, where the ready inputs and buttons take
 * the 44px target below the desktop layout (style.css).
 */
export function RuleRowSkeleton({ touch = false }: { touch?: boolean }) {
  const input = cn("h-9 w-full max-w-xs min-w-0", touch && TOUCH_TARGET_HEIGHT);
  const icon = cn("size-8", touch && TOUCH_TARGET_SIZE);
  return (
    <div className="flex flex-wrap items-center gap-2 rounded-md border p-2">
      <Skeleton className={input} />
      <Skeleton className={input} />
      <Skeleton className={icon} />
      <Skeleton className={icon} />
      <Skeleton className={icon} />
    </div>
  );
}

/** Shared ordered CIDR + description editor for service and datastore ACLs. */
export function IPAllowListEditor({
  entries,
  labels,
  saving,
  onSave,
  clearOnly,
  readOnly,
  emptyBlocks,
  pendingRows,
  disabledReason,
}: {
  entries: IPAllowListEntryDraft[];
  labels: IPAllowListEditorLabels;
  saving: boolean;
  onSave: (entries: IPAllowListEntryDraft[]) => Promise<boolean>;
  /**
   * The workspace plan cannot set inbound rules (below Scale). Existing rules
   * stay listed and enforced; the only write offered is clearing them all.
   */
  clearOnly?: { hint: ReactNode; clearAll: string };
  /**
   * The caller may not change the list at all (a non-Admin on the workspace
   * rules, w9/m388): it is listed as in `clearOnly`, with no write offered.
   * Takes precedence over `clearOnly`.
   */
  readOnly?: { hint: ReactNode };
  /**
   * The list belongs to a service (w8/m157) or a workspace (w9/m388): with no
   * rule every public request is refused, and the allow-all rule is what
   * opens it. `labels.empty` is then the blocked sentence and is shown as a
   * warning, the clear-only view shows `open` for a list nobody restricted,
   * and its button writes `allowAll` (default: a service's single IPv4
   * allow-all rule) instead of an empty list.
   */
  emptyBlocks?: { open: string; allowAll?: IPAllowListEntryDraft[] };
  /**
   * The list is still loading: show this many rule-row placeholders where the
   * rules go and keep every input and action disabled, instead of drawing the
   * empty text ("No IP rules") for a list that is not known to be empty
   * (w4/m315). The caller remounts the editor when the list arrives.
   */
  pendingRows?: number;
  /**
   * Why the caller may not change the list right now (a non-Admin in a
   * protected environment, w4/m594, a role without can_create, w9/m619, or
   * the check still running): the editor keeps its shape with every input and
   * action disabled, and Add, Save and the clear-only action say why.
   */
  disabledReason?: string;
}) {
  const off = disabledReason != null;
  // Keep keys independent of editable values and positions so typing, moving
  // and removing rows preserve the surviving inputs' focus and caret.
  const nextRowID = useRef(entries.length);
  const [draft, setDraft] = useState<IPAllowListRow[]>(() =>
    entries.map((entry, id) => ({ id, entry })),
  );
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
        id: nextRowID.current++,
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

  function cancel() {
    const first = nextRowID.current;
    nextRowID.current += entries.length;
    setDraft(entries.map((entry, offset) => ({ id: first + offset, entry })));
    setCIDR("");
    setDescription("");
    setInvalid(false);
  }

  function move(index: number, delta: -1 | 1) {
    const destination = index + delta;
    if (destination < 0 || destination >= draft.length) return;
    const next = [...draft];
    [next[index], next[destination]] = [next[destination], next[index]];
    setDraft(next);
  }

  const empty = (
    <span
      className={
        emptyBlocks
          ? "text-sm text-destructive"
          : "text-sm text-muted-foreground"
      }
    >
      {labels.empty}
    </span>
  );

  const locked = readOnly ?? clearOnly;
  if (locked) {
    const clear = readOnly ? null : (
      <PermissionTooltip reason={disabledReason}>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() =>
            void onSave(
              emptyBlocks ? (emptyBlocks.allowAll ?? ALLOW_ALL_ENTRIES) : [],
            )
          }
          disabled={saving || off}
        >
          {/* Where an empty list blocks everything, the action adds the
              allow-all rule: a plus, not the trash can of a clear. */}
          {saving ? (
            <Loader2 className="animate-spin" />
          ) : emptyBlocks ? (
            <Plus />
          ) : (
            <Trash2 />
          )}
          {clearOnly?.clearAll}
        </Button>
      </PermissionTooltip>
    );
    return (
      <section className="space-y-2">
        <p className="text-xs text-muted-foreground">{locked.hint}</p>
        {emptyBlocks && isAllowAllRule(entries) ? (
          <span className="text-sm text-muted-foreground">
            {emptyBlocks.open}
          </span>
        ) : entries.length === 0 ? (
          // The warning and its fix as one group with a gap between them:
          // inline, the button butted against the sentence's last word.
          <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
            {empty}
            {emptyBlocks ? clear : null}
          </div>
        ) : (
          <>
            <ul className="space-y-1 font-mono text-sm">
              {entries.map((entry) => (
                <li key={entry.cidrBlock}>
                  {entry.cidrBlock}
                  {entry.description ? (
                    <span className="font-sans text-muted-foreground">
                      {" "}
                      · {entry.description}
                    </span>
                  ) : null}
                </li>
              ))}
            </ul>
            {clear}
          </>
        )}
      </section>
    );
  }

  return (
    <section className="space-y-2">
      <p className="text-xs text-muted-foreground">{labels.hint}</p>
      {pendingRows ? (
        <div className="space-y-2" aria-hidden data-preview="ip-rule-rows">
          {Array.from({ length: pendingRows }, (_, index) => (
            <RuleRowSkeleton key={index} />
          ))}
        </div>
      ) : draft.length === 0 ? (
        empty
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
                disabled={off}
                className="max-w-xs font-mono"
              />
              <Input
                value={entry.description}
                onChange={(event) =>
                  replace(index, "description", event.target.value)
                }
                aria-label={labels.descriptionRule(index + 1)}
                placeholder={labels.descriptionPlaceholder}
                disabled={off}
                className="max-w-xs"
              />
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label={labels.moveUp(entry.cidrBlock)}
                disabled={off || index === 0}
                onClick={() => move(index, -1)}
              >
                <ArrowUp />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label={labels.moveDown(entry.cidrBlock)}
                disabled={off || index === draft.length - 1}
                onClick={() => move(index, 1)}
              >
                <ArrowDown />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label={labels.remove(entry.cidrBlock)}
                disabled={off}
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
          disabled={!!pendingRows || off}
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
          disabled={!!pendingRows || off}
          className="max-w-xs"
        />
        <PermissionTooltip reason={disabledReason}>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={add}
            disabled={off || !cidr.trim()}
          >
            <Plus />
            {labels.add}
          </Button>
        </PermissionTooltip>
        {labels.cancel ? (
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={cancel}
            disabled={saving || (!dirty && !cidr && !description)}
          >
            {labels.cancel}
          </Button>
        ) : null}
        <PermissionTooltip reason={disabledReason}>
          <Button
            type="button"
            size="sm"
            onClick={() => void onSave(draftEntries)}
            disabled={off || !dirty || saving || draftInvalid}
          >
            {saving ? <Loader2 className="animate-spin" /> : null}
            {labels.save}
          </Button>
        </PermissionTooltip>
      </div>
      {invalid || draftInvalid ? (
        <p role="alert" className="text-xs text-destructive">
          {labels.invalid}
        </p>
      ) : null}
    </section>
  );
}
