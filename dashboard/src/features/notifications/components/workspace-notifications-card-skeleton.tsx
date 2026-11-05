import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { cn } from "@/common/lib/utils/utils";

/** The email switch row's frame, shared by the ready card and this skeleton. */
export const EMAIL_ROW_CLASS =
  "flex min-h-16 items-center justify-between gap-4 rounded-md border p-3";

/** One level option's frame, shared by the ready card and this skeleton. */
export const LEVEL_OPTION_CLASS =
  "flex min-h-20 w-full flex-col gap-1 rounded-lg border p-4";

/** A skeleton line for one line of text-sm copy (20px line box). */
function TextLine({ className }: { className?: string }) {
  return (
    <div className={cn("flex h-5 items-center", className)}>
      <Skeleton className="h-4 w-full" />
    </div>
  );
}

/**
 * Workspace Settings → Notifications while the policy loads: header, the email
 * switch row, the three level options and the Save footer, at the ready
 * card's geometry (dashboard/AGENTS.md skeleton rule). Copy that wraps to a
 * second line on a narrow screen gets a second line below `sm` only.
 */
export function WorkspaceNotificationsCardSkeleton() {
  return (
    <Card data-testid="workspace-notifications-skeleton">
      <CardHeader>
        <Skeleton className="h-4 w-28" />
        <div>
          <TextLine className="w-96 max-w-full" />
          <TextLine className="w-40 sm:hidden" />
        </div>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className={EMAIL_ROW_CLASS}>
          <div className="w-full space-y-0.5">
            <Skeleton className="h-3.5 w-36" />
            <div>
              <TextLine className="w-80 max-w-full" />
              <TextLine className="w-24 sm:hidden" />
            </div>
          </div>
          <Skeleton className="h-5 w-9 shrink-0 rounded-full" />
        </div>
        <div className="space-y-3">
          <div className="space-y-1">
            <TextLine className="w-48" />
            <TextLine className="w-72 max-w-full" />
          </div>
          <div className="grid grid-cols-1 gap-3">
            {[true, true, false].map((wraps, i) => (
              <div key={i} className={cn(LEVEL_OPTION_CLASS, "border-border")}>
                <TextLine className="w-48" />
                <div>
                  <TextLine className="w-full" />
                  {wraps ? <TextLine className="w-32 sm:hidden" /> : null}
                </div>
              </div>
            ))}
          </div>
        </div>
      </CardContent>
      <CardFooter className="justify-end border-t">
        <Skeleton className="h-9 w-16" />
      </CardFooter>
    </Card>
  );
}
