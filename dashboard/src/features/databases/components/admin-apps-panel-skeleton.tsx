import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * A card's description holds Render's copy, up to four lines at every width
 * the section lays its cards out in; reserving four keeps the pending frame
 * and the ready one the same height.
 */
export const ADMIN_APP_DESCRIPTION_CLASS =
  "min-h-20 text-sm text-muted-foreground";

/**
 * The Apps section's pending frame: the same card, header and two app cards,
 * each with its title row, four description lines and its action row.
 */
export function AdminAppsPanelSkeleton() {
  const { t } = useTranslations();
  return (
    <Card data-testid="admin-apps-skeleton">
      <CardHeader>
        <CardTitle>{t("databases.adminAppsTitle")}</CardTitle>
        <CardDescription>{t("databases.adminAppsDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="grid gap-4 md:grid-cols-2">
          {[0, 1].map((i) => (
            <div
              key={i}
              className="flex flex-col justify-between gap-4 rounded-lg border p-4"
            >
              <div className="space-y-2">
                <div className="flex min-h-8 items-center">
                  <Skeleton className="h-5 w-24" />
                </div>
                <div className="min-h-20">
                  {[0, 1, 2, 3].map((line) => (
                    <div key={line} className="flex h-5 items-center">
                      <Skeleton
                        className={line === 3 ? "h-3.5 w-2/3" : "h-3.5 w-full"}
                      />
                    </div>
                  ))}
                </div>
              </div>
              <div className="flex items-center gap-3">
                <Skeleton className="h-8 w-28" />
              </div>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
