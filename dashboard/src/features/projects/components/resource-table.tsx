import { Link } from "@tanstack/react-router";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table.tsx";
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
import type { ResourceRow } from "@/features/projects/types";
import type { LifecycleAction, ServiceView } from "@/features/services/types";
import type { PendingLifecycle } from "@/features/services/hooks/use-service-lifecycle";

export interface ResourceTableProps {
  rows: ResourceRow[];
  loading?: boolean;
  servicePending: PendingLifecycle | null;
  onRunServiceAction: (action: LifecycleAction, service: ServiceView) => void;
  onDatabaseDeleted: (id: string) => void;
  onKeyValueDeleted: (id: string) => void;
}

/**
 * The unified Projects page's merged table — one `<Table>` for services,
 * databases, and key-value rows together, with a Type column telling them
 * apart (Render parity, w1/m31 extension). Each row still uses its own
 * kind's status badge + row-actions component so lifecycle behavior can't
 * drift from the standalone `/databases`/`/keyvalue` pages.
 */
export function ResourceTable({
  rows,
  loading,
  servicePending,
  onRunServiceAction,
  onDatabaseDeleted,
  onKeyValueDeleted,
}: ResourceTableProps) {
  const { t } = useTranslations();
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t("projects.colName")}</TableHead>
          <TableHead className="hidden sm:table-cell">
            {t("projects.colType")}
          </TableHead>
          <TableHead>{t("projects.colStatus")}</TableHead>
          <TableHead className="hidden md:table-cell">
            {t("projects.colCreated")}
          </TableHead>
          <TableHead className="w-0 text-right">
            <span className="sr-only">{t("projects.colActions")}</span>
          </TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {loading
          ? Array.from({ length: 2 }).map((_, i) => (
              <ResourceSkeletonRow key={i} />
            ))
          : rows.map((row) => (
              <ResourceTableRow
                key={`${row.kind}:${row.id}`}
                row={row}
                servicePending={servicePending}
                onRunServiceAction={onRunServiceAction}
                onDatabaseDeleted={onDatabaseDeleted}
                onKeyValueDeleted={onKeyValueDeleted}
              />
            ))}
      </TableBody>
    </Table>
  );
}

function ResourceTableRow({
  row,
  servicePending,
  onRunServiceAction,
  onDatabaseDeleted,
  onKeyValueDeleted,
}: {
  row: ResourceRow;
  servicePending: PendingLifecycle | null;
  onRunServiceAction: (action: LifecycleAction, service: ServiceView) => void;
  onDatabaseDeleted: (id: string) => void;
  onKeyValueDeleted: (id: string) => void;
}) {
  if (row.kind === "service" && row.service) {
    const service = row.service;
    return (
      <TableRow>
        <TableCell className="min-w-0 font-medium">
          <Link
            to="/services/$serviceId"
            params={{ serviceId: service.id }}
            className="block max-w-40 truncate hover:underline sm:max-w-56"
          >
            {service.name}
          </Link>
          <div className="mt-1 sm:hidden">
            <ResourceTypeBadge kind="service" />
          </div>
        </TableCell>
        <TableCell className="hidden sm:table-cell">
          <ResourceTypeBadge kind="service" />
        </TableCell>
        <TableCell>
          <ServiceStatusBadge service={service} />
        </TableCell>
        <TableCell className="hidden tabular-nums text-muted-foreground md:table-cell">
          {formatRelativeAge(service.createdAt)}
        </TableCell>
        <TableCell className="text-right">
          <ServiceRowActions
            service={service}
            pending={
              servicePending?.id === service.id ? servicePending.action : null
            }
            onRun={onRunServiceAction}
          />
        </TableCell>
      </TableRow>
    );
  }

  if (row.kind === "database" && row.database) {
    const database = row.database;
    return (
      <TableRow>
        <TableCell className="min-w-0 font-medium">
          <Link
            to="/databases/$databaseId"
            params={{ databaseId: database.id }}
            className="block max-w-40 truncate hover:underline sm:max-w-56"
          >
            {database.name}
          </Link>
          <div className="mt-1 sm:hidden">
            <ResourceTypeBadge kind="database" />
          </div>
        </TableCell>
        <TableCell className="hidden sm:table-cell">
          <ResourceTypeBadge kind="database" />
        </TableCell>
        <TableCell>
          <DatabaseStatusBadge status={database.status} />
        </TableCell>
        <TableCell className="hidden tabular-nums text-muted-foreground md:table-cell">
          {formatRelativeAge(database.createdAt)}
        </TableCell>
        <TableCell className="text-right">
          <DatabaseRowActions
            database={database}
            onDeleted={onDatabaseDeleted}
          />
        </TableCell>
      </TableRow>
    );
  }

  if (row.kind === "keyvalue" && row.keyValue) {
    const keyValue = row.keyValue;
    return (
      <TableRow>
        <TableCell className="min-w-0 font-medium">
          <Link
            to="/keyvalue/$keyValueId"
            params={{ keyValueId: keyValue.id }}
            className="block max-w-40 truncate hover:underline sm:max-w-56"
          >
            {keyValue.name}
          </Link>
          <div className="mt-1 sm:hidden">
            <ResourceTypeBadge kind="keyvalue" />
          </div>
        </TableCell>
        <TableCell className="hidden sm:table-cell">
          <ResourceTypeBadge kind="keyvalue" />
        </TableCell>
        <TableCell>
          <KeyValueStatusBadge status={keyValue.status} />
        </TableCell>
        <TableCell className="hidden tabular-nums text-muted-foreground md:table-cell">
          {formatRelativeAge(keyValue.createdAt)}
        </TableCell>
        <TableCell className="text-right">
          <KeyValueRowActions
            keyValue={keyValue}
            onDeleted={onKeyValueDeleted}
          />
        </TableCell>
      </TableRow>
    );
  }

  return null;
}

function ResourceSkeletonRow() {
  return (
    <TableRow>
      <TableCell>
        <Skeleton className="h-4 w-32" />
      </TableCell>
      <TableCell className="hidden sm:table-cell">
        <Skeleton className="h-5 w-20 rounded-md" />
      </TableCell>
      <TableCell>
        <Skeleton className="h-5 w-16 rounded-md" />
      </TableCell>
      <TableCell className="hidden md:table-cell">
        <Skeleton className="h-4 w-10" />
      </TableCell>
      <TableCell className="text-right">
        <Skeleton className="ml-auto size-8 rounded-md" />
      </TableCell>
    </TableRow>
  );
}
