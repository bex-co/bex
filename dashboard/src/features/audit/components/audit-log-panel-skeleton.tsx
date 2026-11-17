import { Download, LockKeyhole } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Label } from "@/common/components/ui/label";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { PanelCenteredState } from "@/common/components/panel-states";
import {
  ReservedButton,
  ReservedText,
} from "@/common/components/reserved-skeleton";
import { PlanBillingLink } from "@/common/components/plan-billing-link";
import { useTranslations } from "@/common/hooks/use-translations";
import { AUDIT_PAGE_SIZE } from "@/features/audit/lib/audit-events";
import { MAX_EXPORT_DAYS } from "@/features/audit/lib/audit-csv";
import { useAuditPlanKnownMissing } from "@/features/audit/hooks/use-audit-plan-known-missing";

/** The audit table's six column headers, shared by the ready table and its
 *  loading frame so the two never drift apart. */
export function AuditLogTableHeader() {
  const { t } = useTranslations();
  return (
    <TableHeader>
      <TableRow>
        <TableHead>{t("audit.columnTimestamp")}</TableHead>
        <TableHead>{t("audit.columnActor")}</TableHead>
        <TableHead>{t("audit.columnSource")}</TableHead>
        <TableHead>{t("audit.columnAction")}</TableHead>
        <TableHead>{t("audit.columnStatus")}</TableHead>
        <TableHead>{t("audit.columnResource")}</TableHead>
      </TableRow>
    </TableHeader>
  );
}

/** A cell's text line (`text-sm`, 20 px) over its `text-xs` sub-line (16 px),
 *  as AuditEventRow's actor, source and resource cells draw them. */
function TwoLines({ width }: { width: string }) {
  return (
    <>
      <Skeleton className={`h-5 ${width}`} />
      <Skeleton className="h-4 w-20" />
    </>
  );
}

/** An address as wide as most the source cell shows. */
const PLACEHOLDER_ADDRESS = "203.0.113.10";

/**
 * The first page's frame while it loads (w4/684): the six headers, one
 * placeholder per event of a full first page at AuditEventRow's height (its
 * actor, source and resource cells are two lines), and the Load more slot. An active
 * admin workspace fills that page, so the swap moves nothing below the card.
 */
export function AuditLogTableFrame() {
  return (
    <div className="space-y-4" data-testid="audit-log-table-frame">
      <Table>
        <AuditLogTableHeader />
        <TableBody>
          {Array.from({ length: AUDIT_PAGE_SIZE }, (_, row) => (
            <TableRow key={row} data-testid="audit-log-row-placeholder">
              <TableCell>
                <Skeleton className="h-5 w-36" />
              </TableCell>
              <TableCell>
                <TwoLines width="w-32" />
              </TableCell>
              {/* The source cell's address is a font-mono run inside the
                  text-sm line, whose line box is half a pixel taller than a
                  plain one: reserving the same run keeps the row's height. */}
              <TableCell className="max-w-[12rem] text-sm">
                <span className="relative font-mono">
                  <ReservedText>{PLACEHOLDER_ADDRESS}</ReservedText>
                </span>
                <Skeleton className="h-4 w-20" />
              </TableCell>
              <TableCell>
                <Skeleton className="h-5 w-40" />
              </TableCell>
              <TableCell>
                <Skeleton className="h-5 w-16" />
              </TableCell>
              <TableCell>
                <TwoLines width="w-36" />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      <Skeleton className="h-8 w-24" />
    </div>
  );
}

/** The plan notice AuditLogPanel settles to on a workspace below Pro. */
export function AuditLogPlanNotice() {
  const { t } = useTranslations();
  return (
    <PanelCenteredState
      icon={<LockKeyhole />}
      title={t("audit.planTitle")}
      body={
        <>
          {t("audit.planBody")} <PlanBillingLink />
        </>
      }
    />
  );
}

/** AuditLogExport's frame (w3/m305): its static labels and hint, the date
 *  fields' boxes and the Export as CSV button's box, so the row wraps as the
 *  ready one does at every width. */
function AuditLogExportFrame() {
  const { t } = useTranslations();
  return (
    <div className="space-y-2" data-testid="audit-log-export-frame">
      <div className="flex flex-wrap items-end gap-3">
        {[t("audit.exportStart"), t("audit.exportEnd")].map((label) => (
          <div key={label} className="space-y-1">
            <Label>{label}</Label>
            <Skeleton className="h-9 w-40" />
          </div>
        ))}
        <ReservedButton variant="outline">
          <Download />
          {t("audit.exportCSV")}
        </ReservedButton>
      </div>
      <p className="text-xs text-muted-foreground">
        {t("audit.exportHint", { days: MAX_EXPORT_DAYS })}
      </p>
    </div>
  );
}

/** AuditLogPanel's frame while it loads: its static title and description,
 *  the export's frame, then the table frame — or, for a workspace known to be
 *  below Pro, the plan notice the panel settles to (w4/684). */
export function AuditLogPanelSkeleton() {
  const { t } = useTranslations();
  const belowPlan = useAuditPlanKnownMissing();
  return (
    <Card aria-hidden="true">
      <CardHeader>
        <CardTitle>{t("audit.title")}</CardTitle>
        <CardDescription>{t("audit.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        {belowPlan ? (
          <div inert>
            <AuditLogPlanNotice />
          </div>
        ) : (
          <>
            <AuditLogExportFrame />
            <AuditLogTableFrame />
          </>
        )}
      </CardContent>
    </Card>
  );
}
