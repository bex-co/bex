import { DashboardLayout } from "@/common/components/dashboard-layout";
import { PageFrame, PageHeader } from "@/common/components/page-frame";
import {
  Card,
  CardContent,
  CardHeader,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * Breakpoint-matched skeletons for `/sandbox-groups`, `/sandbox-groups/$groupId`,
 * `SandboxGroupCredentialsPanel`, and `CredentialVersionsDialog` (w2/m533 t005).
 * Kept in a skeleton-only module so route pending components stay out of the
 * lazy page chunks (w3/066).
 */

export function SandboxGroupsListContentSkeleton({
  rows = 3,
}: {
  rows?: number;
}) {
  return (
    <div
      data-skeleton-region="sandbox-groups-list"
      className="space-y-4"
      aria-hidden="true"
    >
      {/* Desktop table preview */}
      <div className="hidden overflow-hidden rounded-lg border md:block">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[26%]">
                <Skeleton className="h-3.5 w-16" />
              </TableHead>
              <TableHead className="w-[24%]">
                <Skeleton className="h-3.5 w-10" />
              </TableHead>
              <TableHead className="w-[16%]">
                <Skeleton className="h-3.5 w-14" />
              </TableHead>
              <TableHead className="w-[18%]">
                <Skeleton className="h-3.5 w-24" />
              </TableHead>
              <TableHead className="w-[16%] text-right">
                <Skeleton className="ml-auto h-3.5 w-14" />
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {Array.from({ length: rows }).map((_, idx) => (
              <TableRow key={idx}>
                <TableCell className="py-3.5">
                  <div className="flex items-center gap-2">
                    <Skeleton className="h-4 w-32" />
                    {idx === 0 ? <Skeleton className="h-5 w-16 rounded-full" /> : null}
                  </div>
                </TableCell>
                <TableCell className="py-3.5">
                  <Skeleton className="h-4 w-40" />
                </TableCell>
                <TableCell className="py-3.5">
                  <Skeleton className="h-4 w-16" />
                </TableCell>
                <TableCell className="py-3.5">
                  <Skeleton className="h-4 w-12" />
                </TableCell>
                <TableCell className="py-3.5 text-right">
                  <Skeleton className="ml-auto h-8 w-24" />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {/* Narrow-mobile stacked cards preview (< md, 390px) */}
      <div className="space-y-3 md:hidden">
        {Array.from({ length: rows }).map((_, idx) => (
          <Card key={idx} className="gap-3 py-4">
            <CardContent className="space-y-3 px-4">
              <div className="flex items-center justify-between gap-2">
                <Skeleton className="h-4 w-32" />
                <Skeleton className="h-5 w-16 rounded-full" />
              </div>
              <Skeleton className="h-3.5 w-44" />
              <div className="flex items-center justify-between gap-2 pt-1">
                <Skeleton className="h-3.5 w-28" />
                <Skeleton className="h-8 w-24" />
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}

export function SandboxGroupsListRouteSkeleton() {
  const { t } = useTranslations();
  return (
    <DashboardLayout>
      <PageFrame
        pending="sandbox-groups-list"
        header={
          <PageHeader
            title={t("sandboxGroups.pageTitle")}
            description={t("sandboxGroups.pageDescription")}
            actions={<Skeleton className="h-8 w-36 rounded-md" />}
          />
        }
      >
        <SandboxGroupsListContentSkeleton />
      </PageFrame>
    </DashboardLayout>
  );
}

export function SandboxGroupCredentialsPanelSkeleton({
  rows = 3,
}: {
  rows?: number;
}) {
  return (
    <Card
      data-skeleton-region="credentials-panel"
      className="gap-4"
      aria-hidden="true"
    >
      <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="space-y-1.5">
          <Skeleton className="h-5 w-44" />
          <Skeleton className="h-4 w-72 max-w-full" />
        </div>
        <Skeleton className="h-8 w-32 shrink-0 rounded-md" />
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Desktop table preview */}
        <div className="hidden overflow-hidden rounded-lg border md:block">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-[18%]">
                  <Skeleton className="h-3.5 w-16" />
                </TableHead>
                <TableHead className="w-[22%]">
                  <Skeleton className="h-3.5 w-16" />
                </TableHead>
                <TableHead className="w-[20%]">
                  <Skeleton className="h-3.5 w-20" />
                </TableHead>
                <TableHead className="w-[16%]">
                  <Skeleton className="h-3.5 w-16" />
                </TableHead>
                <TableHead className="w-[10%]">
                  <Skeleton className="h-3.5 w-12" />
                </TableHead>
                <TableHead className="w-[14%] text-right">
                  <Skeleton className="ml-auto h-3.5 w-14" />
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {Array.from({ length: rows }).map((_, idx) => (
                <TableRow key={idx}>
                  <TableCell className="py-3.5">
                    <div className="space-y-1">
                      <Skeleton className="h-4 w-28" />
                      <Skeleton className="h-3 w-12" />
                    </div>
                  </TableCell>
                  <TableCell className="py-3.5">
                    <Skeleton className="h-4 w-36" />
                  </TableCell>
                  <TableCell className="py-3.5">
                    <Skeleton className="h-4 w-32" />
                  </TableCell>
                  <TableCell className="py-3.5">
                    <div className="flex flex-wrap gap-1">
                      <Skeleton className="h-5 w-12 rounded-full" />
                      <Skeleton className="h-5 w-14 rounded-full" />
                    </div>
                  </TableCell>
                  <TableCell className="py-3.5">
                    <Skeleton className="h-5 w-10 rounded-full" />
                  </TableCell>
                  <TableCell className="py-3.5 text-right">
                    <div className="flex justify-end gap-1.5">
                      <Skeleton className="h-8 w-8 rounded-md" />
                      <Skeleton className="h-8 w-8 rounded-md" />
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>

        {/* Narrow-mobile stacked cards preview (< md, 390px) */}
        <div className="space-y-3 md:hidden">
          {Array.from({ length: rows }).map((_, idx) => (
            <div
              key={idx}
              className="space-y-3 rounded-lg border p-4"
            >
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0 space-y-1">
                  <Skeleton className="h-4 w-32" />
                  <Skeleton className="h-3.5 w-40" />
                </div>
                <Skeleton className="h-5 w-10 shrink-0 rounded-full" />
              </div>
              <div className="flex flex-wrap gap-1.5">
                <Skeleton className="h-5 w-24 rounded-full" />
                <Skeleton className="h-5 w-16 rounded-full" />
                <Skeleton className="h-5 w-12 rounded-full" />
              </div>
              <div className="flex items-center justify-between gap-2 border-t pt-2.5">
                <Skeleton className="h-3.5 w-16" />
                <div className="flex gap-1.5">
                  <Skeleton className="h-8 w-20 rounded-md" />
                  <Skeleton className="h-8 w-16 rounded-md" />
                </div>
              </div>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}

export function SandboxGroupDetailRouteSkeleton() {
  return (
    <DashboardLayout>
      <PageFrame
        pending="sandbox-group-detail"
        header={
          <PageHeader
            leading={<Skeleton className="h-8 w-8 rounded-md" />}
            title={<Skeleton className="h-6 w-44" />}
            titleAddon={<Skeleton className="h-5 w-16 rounded-full" />}
            description={<Skeleton className="h-4 w-56" />}
          />
        }
      >
        {/* Group metadata summary row */}
        <div
          data-skeleton-region="group-metadata"
          className="grid grid-cols-1 gap-4 sm:grid-cols-3"
          aria-hidden="true"
        >
          {Array.from({ length: 3 }).map((_, idx) => (
            <Card key={idx} className="gap-1.5 py-4">
              <CardHeader className="px-4 py-0">
                <Skeleton className="h-3.5 w-20" />
                <Skeleton className="h-5 w-28" />
              </CardHeader>
            </Card>
          ))}
        </div>
        <SandboxGroupCredentialsPanelSkeleton />
      </PageFrame>
    </DashboardLayout>
  );
}

export function CredentialVersionsSkeleton({ rows = 3 }: { rows?: number }) {
  return (
    <div
      data-skeleton-region="credential-versions-list"
      className="space-y-3"
      aria-hidden="true"
    >
      {Array.from({ length: rows }).map((_, idx) => (
        <div
          key={idx}
          className="flex flex-col gap-2 rounded-lg border p-3 sm:flex-row sm:items-center sm:justify-between"
        >
          <div className="space-y-1.5">
            <div className="flex items-center gap-2">
              <Skeleton className="h-5 w-10 rounded-full" />
              {idx === 0 ? (
                <Skeleton className="h-5 w-16 rounded-full" />
              ) : null}
            </div>
            <Skeleton className="h-3.5 w-44" />
            <Skeleton className="h-3 w-36" />
          </div>
          <Skeleton className="h-8 w-20 shrink-0 rounded-md" />
        </div>
      ))}
    </div>
  );
}
