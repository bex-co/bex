import { useId, useMemo, useState, type KeyboardEvent } from "react";
import { ChevronDown, Loader2, Plus, Trash2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { LabelError } from "@/common/components/label-error";
import { RemovableChip } from "@/common/components/removable-chip";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { isValidLabel } from "@/common/lib/utils/resource-label";
import { useEnvironments } from "@/features/environments/hooks/use-environments";
import { useProjects } from "@/features/projects/hooks/use-projects";
import { useDatabases } from "@/features/databases/hooks/use-databases";
import { useKeyValues } from "@/features/keyvalue/hooks/use-key-values";
import { useServices } from "@/features/services/hooks/use-services";
import {
  isStaticSiteType,
  isWebServiceType,
} from "@/features/services/lib/service-type";
import type { SavedSearch } from "@/features/projects/hooks/use-saved-searches";
import { useSavedSearchMutations } from "@/features/projects/hooks/use-saved-search-mutations";
import { SavedSearchType } from "@/graphql/definitions";
import { RANGE_LABEL_KEYS, RANGE_PRESETS } from "@/features/metrics/lib/range";
import {
  VIEW_FIELD_LABEL_KEYS,
  VIEW_FIELDS_BY_TYPE,
  documentFromRows,
  incompleteRows,
  canAddFilterRow,
  newFilterRow,
  rowsFromDocument,
  viewFieldIsSingle,
  viewFieldOperators,
  viewFieldOptions,
  viewValueKind,
  withField,
  type ViewField,
  type ViewFilterRow,
  type ViewOperator,
} from "../lib/view-filters";

/** What the modal edits: a new view of a type, or an existing one. */
export type EditViewTarget =
  | { kind: "create"; type: SavedSearchType; name: string }
  | { kind: "edit"; view: SavedSearch };

export interface EditViewDialogProps {
  projectId: string;
  /** Null keeps the dialog closed. */
  target: EditViewTarget | null;
  onOpenChange: (open: boolean) => void;
  /** The view as saved, once Done has written it. */
  onSaved?: (view: SavedSearch) => void;
}

/**
 * Render's **Edit view** modal (w3/m507): the view's Name, its Filters as
 * rows of Field, Operator and Values with a trash button each and an add
 * button, the notice that a change is everyone's on the project, then Close
 * and Done. New View opens the same modal for the type it picked, titled
 * "New view", and nothing is created until Done; Close discards either.
 */
export function EditViewDialog({
  projectId,
  target,
  onOpenChange,
  onSaved,
}: EditViewDialogProps) {
  return (
    <Dialog open={target !== null} onOpenChange={onOpenChange}>
      {target ? (
        <DialogContent
          className="sm:max-w-2xl"
          // Render's footer Close is the modal's close; Escape still works.
          showCloseButton={false}
        >
          <EditViewForm
            // A new target starts from its own state, never the last one's.
            key={target.kind === "edit" ? target.view.id : `new-${target.type}`}
            projectId={projectId}
            target={target}
            onClose={() => onOpenChange(false)}
            onSaved={onSaved}
          />
        </DialogContent>
      ) : null}
    </Dialog>
  );
}

function EditViewForm({
  projectId,
  target,
  onClose,
  onSaved,
}: {
  projectId: string;
  target: EditViewTarget;
  onClose: () => void;
  onSaved?: (view: SavedSearch) => void;
}) {
  const { t } = useTranslations();
  const nameId = useId();
  const type = target.kind === "edit" ? target.view.type : target.type;
  const [name, setName] = useState(
    target.kind === "edit" ? target.view.name : target.name,
  );
  const [rows, setRows] = useState<ViewFilterRow[]>(() =>
    target.kind === "edit" ? rowsFromDocument(target.view.filters, type) : [],
  );
  const [attempted, setAttempted] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  const { create, update, busy } = useSavedSearchMutations(projectId);
  const choices = useValueChoices(projectId, type);

  const incomplete = incompleteRows(rows);
  const canAdd = canAddFilterRow(type, rows);
  const saving = busy === "create" || busy === "update";

  async function done() {
    setAttempted(true);
    if (!isValidLabel(name) || incomplete.size > 0 || saving) return;
    setFailure(null);
    const filters = documentFromRows(rows);
    try {
      const saved =
        target.kind === "edit"
          ? await update(target.view.id, name.trim(), filters)
          : await create(type, name.trim(), filters);
      onSaved?.(saved);
      onClose();
    } catch (err) {
      setFailure(mutationErrorMessage(err, t("logs.viewSaveFailed")));
    }
  }

  const setRow = (key: string, change: (row: ViewFilterRow) => ViewFilterRow) =>
    setRows((current) =>
      current.map((row) => (row.key === key ? change(row) : row)),
    );

  return (
    <>
      <DialogHeader>
        <DialogTitle>
          {target.kind === "edit"
            ? t("logs.editViewTitle")
            : t("logs.newViewTitle")}
        </DialogTitle>
      </DialogHeader>

      <div className="space-y-2">
        <Label htmlFor={nameId}>{t("logs.viewNameLabel")}</Label>
        <Input
          id={nameId}
          value={name}
          autoComplete="off"
          aria-invalid={attempted && !isValidLabel(name)}
          aria-describedby={`${nameId}-error`}
          onChange={(event) => setName(event.target.value)}
        />
        <LabelError id={`${nameId}-error`} value={name} />
        {attempted && name.trim() === "" ? (
          <p role="alert" className="text-xs text-destructive">
            {t("logs.viewNameRequired")}
          </p>
        ) : null}
      </div>

      <div className="space-y-3">
        <div className="flex items-center justify-between gap-2">
          <h3 className="text-sm font-medium">{t("logs.viewFiltersLabel")}</h3>
          <Button
            type="button"
            variant="outline"
            size="icon"
            className="size-8"
            aria-label={t("logs.viewAddFilter")}
            title={t("logs.viewAddFilter")}
            disabled={!canAdd}
            onClick={() =>
              setRows((current) => {
                const next = newFilterRow(type, current);
                return next ? [...current, next] : current;
              })
            }
          >
            <Plus />
          </Button>
        </div>
        {rows.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            {t("logs.viewNoFilters")}
          </p>
        ) : (
          <ul className="space-y-3">
            {rows.map((row) => (
              <FilterRow
                key={row.key}
                row={row}
                type={type}
                usedFields={rows
                  .filter((other) => other.key !== row.key)
                  .map((other) => other.field)}
                choices={choices}
                missingValues={attempted && incomplete.has(row.key)}
                onChange={(change) => setRow(row.key, change)}
                onRemove={() =>
                  setRows((current) =>
                    current.filter((other) => other.key !== row.key),
                  )
                }
              />
            ))}
          </ul>
        )}
        <DialogDescription>{t("logs.viewSharedNotice")}</DialogDescription>
      </div>

      {failure ? (
        <p role="alert" className="text-sm text-destructive">
          {failure}
        </p>
      ) : null}

      <DialogFooter>
        <Button variant="outline" onClick={onClose} disabled={saving}>
          {t("logs.viewClose")}
        </Button>
        <Button onClick={() => void done()} disabled={saving}>
          {saving ? <Loader2 className="animate-spin" /> : null}
          {t("logs.viewDone")}
        </Button>
      </DialogFooter>
    </>
  );
}

interface ValueChoice {
  value: string;
  label: string;
}

interface ValueChoices {
  environments: ValueChoice[];
  resources: ValueChoice[];
}

/**
 * The project's environments and the resources a view of this type can read
 * (Request Logs: its web services; static sites have no log), for the
 * Environment and Resource pickers.
 */
function useValueChoices(
  projectId: string,
  type: SavedSearchType,
): ValueChoices {
  const { environments } = useEnvironments(projectId, { poll: false });
  const { projects } = useProjects({ poll: false });
  // The modal's own one-off reads: the page already follows the project.
  const { services } = useServices({ poll: false });
  const { databases } = useDatabases({ poll: false });
  const { keyValues } = useKeyValues({ poll: false });
  const project = projects.find((p) => p.id === projectId);
  return useMemo(() => {
    const members = new Set([
      ...(project?.serviceIds ?? []),
      ...(project?.databaseIds ?? []),
      ...(project?.keyValueIds ?? []),
    ]);
    const requestsOnly = type === SavedSearchType.RequestLogs;
    // A static site has no application log, but Service Metrics charts its
    // requests and bandwidth (w3/m508).
    const staticSites = type === SavedSearchType.Metrics;
    const resources: ValueChoice[] = [
      ...services
        .filter(
          (s) =>
            members.has(s.id) && (staticSites || !isStaticSiteType(s.type)),
        )
        .filter((s) => !requestsOnly || isWebServiceType(s.type))
        .map((s) => ({ value: s.id, label: s.name })),
      ...(requestsOnly
        ? []
        : [...databases, ...keyValues]
            .filter((d) => members.has(d.id))
            .map((d) => ({ value: d.id, label: d.name }))),
    ];
    return {
      environments: environments.map((e) => ({ value: e.id, label: e.name })),
      resources,
    };
  }, [environments, project, services, databases, keyValues, type]);
}

function FilterRow({
  row,
  type,
  usedFields,
  choices,
  missingValues,
  onChange,
  onRemove,
}: {
  row: ViewFilterRow;
  type: SavedSearchType;
  usedFields: ViewField[];
  choices: ValueChoices;
  missingValues: boolean;
  onChange: (change: (row: ViewFilterRow) => ViewFilterRow) => void;
  onRemove: () => void;
}) {
  const { t } = useTranslations();
  const fieldLabel = t(VIEW_FIELD_LABEL_KEYS[row.field]);
  const fields = VIEW_FIELDS_BY_TYPE[type].filter(
    (field) => field === row.field || !usedFields.includes(field),
  );
  const operators = viewFieldOperators(row.field);
  const errorId = `${row.key}-values-error`;
  return (
    <li className="flex flex-wrap items-start gap-2">
      <Select
        value={row.field}
        onValueChange={(field) =>
          onChange((current) => withField(current, field as ViewField))
        }
      >
        <SelectTrigger
          className="w-40 max-sm:flex-1"
          aria-label={t("logs.viewFilterField")}
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {fields.map((field) => (
            <SelectItem key={field} value={field}>
              {t(VIEW_FIELD_LABEL_KEYS[field])}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select
        value={row.operator}
        disabled={operators.length < 2}
        onValueChange={(operator) =>
          onChange((current) => ({
            ...current,
            operator: operator as ViewOperator,
          }))
        }
      >
        <SelectTrigger
          className="w-36 max-sm:flex-1"
          aria-label={t("logs.viewFilterOperator")}
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {operators.map((operator) => (
            <SelectItem key={operator} value={operator}>
              {operatorLabel(t, operator, viewFieldIsSingle(row.field))}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <div className="min-w-48 flex-1 space-y-1">
        <ValuesPicker
          row={row}
          choices={choices}
          label={t("logs.viewFilterValuesFor", { field: fieldLabel })}
          invalid={missingValues}
          errorId={errorId}
          onValues={(values) => onChange((current) => ({ ...current, values }))}
        />
        {missingValues ? (
          <p id={errorId} role="alert" className="text-xs text-destructive">
            {t("logs.viewValuesRequired")}
          </p>
        ) : null}
      </div>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="size-9 shrink-0"
        aria-label={t("logs.viewRemoveFilter", { field: fieldLabel })}
        title={t("logs.viewRemoveFilter", { field: fieldLabel })}
        onClick={onRemove}
      >
        <Trash2 />
      </Button>
    </li>
  );
}

function operatorLabel(
  t: ReturnType<typeof useTranslations>["t"],
  operator: ViewOperator,
  single: boolean,
): string {
  if (operator === "IS_NOT") return t("logs.viewOperatorIsNotOneOf");
  return single ? t("logs.viewOperatorIs") : t("logs.viewOperatorIsOneOf");
}

/**
 * The options plus any stored value they no longer list (a deleted
 * environment or resource), shown as itself rather than read as unset.
 */
function withStale(options: ValueChoice[], values: string[]): ValueChoice[] {
  const stale = values.filter((v) => !options.some((o) => o.value === v));
  return stale.length === 0
    ? options
    : [...options, ...stale.map((v) => ({ value: v, label: v }))];
}

function ValuesPicker({
  row,
  choices,
  label,
  invalid,
  errorId,
  onValues,
}: {
  row: ViewFilterRow;
  choices: ValueChoices;
  label: string;
  invalid: boolean;
  errorId: string;
  onValues: (values: string[]) => void;
}) {
  const { t } = useTranslations();
  const kind = viewValueKind(row.field);
  const describedBy = invalid ? errorId : undefined;

  if (kind === "environment" || kind === "timeRange") {
    const options: ValueChoice[] =
      kind === "environment"
        ? choices.environments
        : RANGE_PRESETS.map((preset) => ({
            value: preset.id,
            label: t(RANGE_LABEL_KEYS[preset.id]),
          }));
    const current = row.values[0];
    const shown = withStale(options, row.values);
    return (
      <Select
        value={current ?? ""}
        onValueChange={(value) => onValues([value])}
      >
        <SelectTrigger
          className="w-full"
          aria-label={label}
          aria-invalid={invalid}
          aria-describedby={describedBy}
        >
          <SelectValue placeholder={t("logs.viewValuesPlaceholder")} />
        </SelectTrigger>
        <SelectContent>
          {shown.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    );
  }

  if (kind === "resource" || kind === "options") {
    const options: ValueChoice[] =
      kind === "resource"
        ? choices.resources
        : viewFieldOptions(row.field).map(({ value, labelKey }) => ({
            value,
            label: labelKey ? t(labelKey) : value,
          }));
    const shown = withStale(options, row.values);
    const labelOf = (value: string) =>
      shown.find((o) => o.value === value)?.label ?? value;
    return (
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            type="button"
            variant="outline"
            className="w-full justify-between font-normal"
            aria-label={label}
            aria-invalid={invalid}
            aria-describedby={describedBy}
          >
            <span
              className={
                row.values.length > 0
                  ? "min-w-0 truncate"
                  : "min-w-0 truncate text-muted-foreground"
              }
            >
              {row.values.length > 0
                ? row.values.map(labelOf).join(", ")
                : t("logs.viewValuesPlaceholder")}
            </span>
            <ChevronDown className="opacity-50" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="max-h-72 overflow-y-auto">
          {shown.length === 0 ? (
            <p className="px-2 py-1.5 text-sm text-muted-foreground">
              {t("logs.viewNoResources")}
            </p>
          ) : (
            shown.map((option) => (
              <DropdownMenuCheckboxItem
                key={option.value}
                checked={row.values.includes(option.value)}
                // A multi-select: the menu stays open between picks.
                onSelect={(event) => event.preventDefault()}
                onCheckedChange={(checked) =>
                  onValues(
                    checked
                      ? [...row.values, option.value]
                      : row.values.filter((v) => v !== option.value),
                  )
                }
              >
                {option.label}
              </DropdownMenuCheckboxItem>
            ))
          )}
        </DropdownMenuContent>
      </DropdownMenu>
    );
  }

  return (
    <TypedValues
      values={row.values}
      label={label}
      invalid={invalid}
      describedBy={describedBy}
      onValues={onValues}
    />
  );
}

/**
 * Values that are typed rather than picked (an instance, a search term, a
 * status code, a host, a path): Enter or a comma adds the text as a value,
 * each value a chip with its own remove button.
 */
function TypedValues({
  values,
  label,
  invalid,
  describedBy,
  onValues,
}: {
  values: string[];
  label: string;
  invalid: boolean;
  describedBy: string | undefined;
  onValues: (values: string[]) => void;
}) {
  const { t } = useTranslations();
  const [draft, setDraft] = useState("");
  const commit = () => {
    const value = draft.trim();
    if (value !== "" && !values.includes(value)) onValues([...values, value]);
    setDraft("");
  };
  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Enter" || event.key === ",") {
      event.preventDefault();
      commit();
    } else if (event.key === "Backspace" && draft === "" && values.length > 0) {
      onValues(values.slice(0, -1));
    }
  };
  return (
    <div className="space-y-2">
      <Input
        value={draft}
        aria-label={label}
        aria-invalid={invalid}
        aria-describedby={describedBy}
        placeholder={t("logs.viewValueTypeHint")}
        autoComplete="off"
        onChange={(event) => setDraft(event.target.value)}
        onKeyDown={onKeyDown}
        onBlur={commit}
      />
      {values.length > 0 ? (
        <div className="flex flex-wrap gap-1.5">
          {values.map((value) => (
            <RemovableChip
              key={value}
              className="font-mono"
              removeLabel={t("logs.viewValueRemove", { value })}
              onRemove={() => onValues(values.filter((v) => v !== value))}
            >
              {value}
            </RemovableChip>
          ))}
        </div>
      ) : null}
    </div>
  );
}
