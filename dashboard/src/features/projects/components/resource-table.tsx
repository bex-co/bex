import { Link } from "@tanstack/react-router";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table.tsx";
import { Checkbox } from "@/common/components/ui/checkbox";
import { Skeleton } from "@/common/components/ui/skeleton.tsx";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatRelativeAge } from "@/features/services/lib/format";
import { ServiceStatusBadge } from "@/features/services/components/service-status-badge";
import { ServiceRowActions } from "@/features/services/components/service-row-actions";
import { DatabaseStatusBadge } from "@/features/databases/components/database-status-badge";
import { DatabaseRowActions } from "@/features/databases/components/database-row-actions";
import { KeyValueStatusBadge } from "@/features/keyvalue/components/key-value-status-badge";
import { KeyValueRowActions } from "@/features/keyvalue/components/key-value-row-actions";
import { ResourceTypeBadge } from "@/features/projects/components/resource-type-badge";
import {
  resourceSelectionKey,
  type ResourceRow,
} from "@/features/projects/types";
import type {
  PendingLifecycle,
  RunServiceAction,
} from "@/features/services/hooks/use-service-lifecycle";

export interface ResourceTableProps {
  rows: ResourceRow[];
  loading?: boolean;
  servicePending: PendingLifecycle | null;
  onRunServiceAction: RunServiceAction;
  onDatabaseDeleted: (id: string) => void;
  onKeyValueDeleted: (id: string) => void;
  /** Project Environment views use Render's operational metadata columns. */
  projectMetadata?: boolean;
  /** Supplying both selection props enables accessible row/select-visible-all checkboxes. */
  selectedKeys?: ReadonlySet<string>;
  onSelectedKeysChange?: (keys: Set<string>) => void;
}

/** Render-shaped mixed-resource table backed only by authoritative list facts. */
export function ResourceTable({
  rows,
  loading,
  servicePending,
  onRunServiceAction,
  onDatabaseDeleted,
  onKeyValueDeleted,
  projectMetadata = false,
  selectedKeys,
  onSelectedKeysChange,
}: ResourceTableProps) {
  const { t } = useTranslations();
  const selectable = selectedKeys != null && onSelectedKeysChange != null;
  const visibleKeys = rows.map(resourceSelectionKey);
  const selectedVisibleCount = selectable
    ? visibleKeys.filter((key) => selectedKeys.has(key)).length
    : 0;
  const allVisibleSelected =
    rows.length > 0 && selectedVisibleCount === rows.length;
  const someVisibleSelected = selectedVisibleCount > 0 && !allVisibleSelected;

  function selectVisible(checked: boolean) {
    if (!selectable) return;
    const next = new Set(selectedKeys);
    for (const key of visibleKeys) {
      if (checked) next.add(key);
      else next.delete(key);
    }
    onSelectedKeysChange(next);
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          {selectable ? (
            <TableHead className="w-10">
              <Checkbox
                aria-label={t("projects.selectAllVisible")}
                checked={
                  someVisibleSelected ? "indeterminate" : allVisibleSelected
                }
                disabled={rows.length === 0}
                onCheckedChange={(checked) => selectVisible(checked === true)}
              />
            </TableHead>
          ) : null}
          <TableHead>{t("projects.colName")}</TableHead>
          {!projectMetadata ? (
            <TableHead className="hidden sm:table-cell">
              {t("projects.colType")}
            </TableHead>
          ) : null}
          <TableHead>{t("projects.colStatus")}</TableHead>
          {projectMetadata ? (
            <>
              <TableHead className="hidden md:table-cell">
                {t("projects.colRuntime")}
              </TableHead>
              <TableHead className="hidden lg:table-cell">
                {t("projects.colRegion")}
              </TableHead>
              <TableHead className="hidden sm:table-cell">
                {t("projects.colUpdated")}
              </TableHead>
            </>
          ) : (
            <TableHead className="hidden md:table-cell">
              {t("projects.colCreated")}
            </TableHead>
          )}
          <TableHead className="w-0 text-right">
            <span className="sr-only">{t("projects.colActions")}</span>
          </TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {loading
          ? Array.from({ length: 2 }).map((_, i) => (
              <ResourceSkeletonRow
                key={i}
                selectable={selectable}
                projectMetadata={projectMetadata}
              />
            ))
          : rows.map((row) => {
              const key = resourceSelectionKey(row);
              return (
                <ResourceTableRow
                  key={key}
                  row={row}
                  projectMetadata={projectMetadata}
                  checked={selectedKeys?.has(key) ?? false}
                  onCheckedChange={
                    selectable
                      ? (checked) => {
                          const next = new Set(selectedKeys);
                          if (checked) next.add(key);
                          else next.delete(key);
                          onSelectedKeysChange(next);
                        }
                      : undefined
                  }
                  servicePending={servicePending}
                  onRunServiceAction={onRunServiceAction}
                  onDatabaseDeleted={onDatabaseDeleted}
                  onKeyValueDeleted={onKeyValueDeleted}
                />
              );
            })}
      </TableBody>
    </Table>
  );
}

function ResourceTableRow({
  row,
  checked,
  projectMetadata,
  onCheckedChange,
  servicePending,
  onRunServiceAction,
  onDatabaseDeleted,
  onKeyValueDeleted,
}: {
  row: ResourceRow;
  checked: boolean;
  projectMetadata: boolean;
  onCheckedChange?: (checked: boolean) => void;
  servicePending: PendingLifecycle | null;
  onRunServiceAction: RunServiceAction;
  onDatabaseDeleted: (id: string) => void;
  onKeyValueDeleted: (id: string) => void;
}) {
  const { t } = useTranslations();
  return (
    <TableRow data-state={checked ? "selected" : undefined}>
      {onCheckedChange ? (
        <TableCell className="w-10">
          <Checkbox
            aria-label={t("projects.selectResource", { name: row.name })}
            checked={checked}
            onCheckedChange={(value) => onCheckedChange(value === true)}
          />
        </TableCell>
      ) : null}
      <TableCell className="min-w-0 font-medium">
        <ResourceLink row={row} />
        <div className={projectMetadata ? "mt-1" : "mt-1 sm:hidden"}>
          <ResourceTypeBadge kind={row.kind} />
        </div>
      </TableCell>
      {!projectMetadata ? (
        <TableCell className="hidden sm:table-cell">
          <ResourceTypeBadge kind={row.kind} />
        </TableCell>
      ) : null}
      <TableCell>
        <ResourceStatus row={row} />
      </TableCell>
      {projectMetadata ? (
        <>
          <TableCell className="hidden text-muted-foreground md:table-cell">
            {row.runtime ?? <UnknownValue />}
          </TableCell>
          <TableCell className="hidden text-muted-foreground lg:table-cell">
            {row.region ?? <UnknownValue />}
          </TableCell>
          <TableCell className="hidden tabular-nums text-muted-foreground sm:table-cell">
            {row.updatedAt ? (
              formatRelativeAge(row.updatedAt)
            ) : (
              <UnknownValue />
            )}
          </TableCell>
        </>
      ) : (
        <TableCell className="hidden tabular-nums text-muted-foreground md:table-cell">
          {formatRelativeAge(row.createdAt)}
        </TableCell>
      )}
      <TableCell className="text-right">
        <ResourceActions
          row={row}
          servicePending={servicePending}
          onRunServiceAction={onRunServiceAction}
          onDatabaseDeleted={onDatabaseDeleted}
          onKeyValueDeleted={onKeyValueDeleted}
        />
      </TableCell>
    </TableRow>
  );
}

function ResourceLink({ row }: { row: ResourceRow }) {
  const className = "block max-w-40 truncate hover:underline sm:max-w-56";
  if (row.kind === "service") {
    return (
      <Link
        to="/services/$serviceId"
        params={{ serviceId: row.id }}
        className={className}
      >
        {row.name}
      </Link>
    );
  }
  if (row.kind === "database") {
    return (
      <Link
        to="/databases/$databaseId"
        params={{ databaseId: row.id }}
        className={className}
      >
        {row.name}
      </Link>
    );
  }
  if (row.kind === "keyvalue") {
    return (
      <Link
        to="/keyvalue/$keyValueId"
        params={{ keyValueId: row.id }}
        className={className}
      >
        {row.name}
      </Link>
    );
  }
  return (
    <Link
      to="/env-groups/$groupId"
      params={{ groupId: row.id }}
      className={className}
    >
      {row.name}
    </Link>
  );
}

function ResourceStatus({ row }: { row: ResourceRow }) {
  if (row.kind === "service" && row.service)
    return <ServiceStatusBadge service={row.service} />;
  if (row.kind === "database" && row.database)
    return <DatabaseStatusBadge database={row.database} />;
  if (row.kind === "keyvalue" && row.keyValue)
    return <KeyValueStatusBadge keyValue={row.keyValue} />;
  return <UnknownValue />;
}

function ResourceActions({
  row,
  servicePending,
  onRunServiceAction,
  onDatabaseDeleted,
  onKeyValueDeleted,
}: Pick<
  ResourceTableProps,
  | "servicePending"
  | "onRunServiceAction"
  | "onDatabaseDeleted"
  | "onKeyValueDeleted"
> & { row: ResourceRow }) {
  if (row.kind === "service" && row.service) {
    return (
      <ServiceRowActions
        service={row.service}
        pending={servicePending?.id === row.id ? servicePending.action : null}
        onRun={onRunServiceAction}
      />
    );
  }
  if (row.kind === "database" && row.database) {
    return (
      <DatabaseRowActions
        database={row.database}
        onDeleted={onDatabaseDeleted}
      />
    );
  }
  if (row.kind === "keyvalue" && row.keyValue) {
    return (
      <KeyValueRowActions
        keyValue={row.keyValue}
        onDeleted={onKeyValueDeleted}
      />
    );
  }
  return null;
}

function UnknownValue() {
  const { t } = useTranslations();
  return <span aria-label={t("projects.metadataUnavailable")}>—</span>;
}

function ResourceSkeletonRow({
  selectable,
  projectMetadata = false,
}: {
  selectable: boolean;
  projectMetadata?: boolean;
}) {
  return (
    <TableRow>
      {selectable ? (
        <TableCell>
          <Skeleton className="size-4" />
        </TableCell>
      ) : null}
      <TableCell>
        <Skeleton className="h-4 w-32" />
      </TableCell>
      {!projectMetadata ? (
        <TableCell className="hidden sm:table-cell">
          <Skeleton className="h-5 w-20 rounded-md" />
        </TableCell>
      ) : null}
      <TableCell>
        <Skeleton className="h-5 w-16 rounded-md" />
      </TableCell>
      {projectMetadata ? (
        <>
          <TableCell className="hidden md:table-cell">
            <Skeleton className="h-4 w-16" />
          </TableCell>
          <TableCell className="hidden lg:table-cell">
            <Skeleton className="h-4 w-12" />
          </TableCell>
          <TableCell className="hidden sm:table-cell">
            <Skeleton className="h-4 w-10" />
          </TableCell>
        </>
      ) : (
        <TableCell className="hidden md:table-cell">
          <Skeleton className="h-4 w-10" />
        </TableCell>
      )}
      <TableCell className="text-right">
        <Skeleton className="ml-auto size-8 rounded-md" />
      </TableCell>
    </TableRow>
  );
}
