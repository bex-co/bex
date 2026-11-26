import { Skeleton } from "@/common/components/ui/skeleton";
import { AVATAR_ROW_CONTROL_CLASS } from "@/features/avatars/components/avatar-row-layout";

/** The Avatar row's control while it loads: the 64 px circle over the Edit
 *  button's box, as `AvatarEditor` lays them out. */
export function AvatarEditorSkeleton() {
  return (
    <div className={AVATAR_ROW_CONTROL_CLASS}>
      <Skeleton className="size-16 rounded-full" />
      <Skeleton className="h-8 w-17" />
    </div>
  );
}
