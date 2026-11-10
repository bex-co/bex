import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { PanelTableSkeleton } from "@/common/components/panel-states";

/** RegistryCredentialsPanel's frame while it loads: title, description, the
 *  Add button and two rows. */
export function RegistryCredentialsPanelSkeleton() {
  return (
    <Card aria-hidden="true">
      <CardHeader className="gap-2">
        <Skeleton className="h-5 w-44" />
        <Skeleton className="h-4 w-80 max-w-full" />
        <CardAction>
          <Skeleton className="h-8 w-32" />
        </CardAction>
      </CardHeader>
      <CardContent>
        <PanelTableSkeleton />
      </CardContent>
    </Card>
  );
}
