import { Skeleton } from "@/common/components/ui/skeleton";

/**
 * The transcript-row-shaped skeleton shared by every transient state between
 * the route-level frame and the real conversation: the dynamic-import wait
 * (session-conversation.tsx), the sandbox-provisioning gate
 * (session-chat-column.tsx), and the resume-in-flight window
 * (session-conversation-impl.tsx). Mirrors the real transcript's own row
 * shapes — assistant prose is unboxed; user turns are a right-aligned bubble
 * followed by a circular avatar. Presentation-only (no text/i18n — matches
 * detail-skeletons.tsx); callers that need an accessible status announcement
 * add their own `role="status"`/`aria-label`.
 */
export function ConversationSkeleton() {
  return (
    <div aria-hidden="true" className="space-y-2.5">
      <div className="space-y-2 py-1">
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-[88%]" />
        <Skeleton className="h-4 w-[68%]" />
      </div>
      <div className="flex w-full justify-end gap-2 pl-8">
        <Skeleton className="h-10 w-64 max-w-[92%] rounded-xl rounded-br-md sm:max-w-md" />
        <Skeleton className="size-8 shrink-0 rounded-full" />
      </div>
      <div className="space-y-2 py-1">
        <Skeleton className="h-4 w-[94%]" />
        <Skeleton className="h-4 w-[76%]" />
        <Skeleton className="h-4 w-[52%]" />
      </div>
    </div>
  );
}
