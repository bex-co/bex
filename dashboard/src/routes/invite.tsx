import { createFileRoute } from "@tanstack/react-router";
import { InviteFallbackPage } from "@/features/invites/invite-fallback-page";
import { translatedTitleHead } from "@/common/lib/document-head";
import { InviteRouteSkeleton } from "@/common/components/route-skeletons";

export const Route = createFileRoute("/invite")({
  component: InviteRoute,
  pendingComponent: InvitePendingRoute,
  head: ({ match }) => {
    const translated = translatedTitleHead("invites.title", match);
    return {
      ...translated,
      meta: [
        ...(translated.meta ?? []),
        { name: "referrer", content: "no-referrer" },
        { name: "robots", content: "noindex, nofollow" },
      ],
    };
  },
});

function InviteRoute() {
  const { session } = Route.useRouteContext();
  const navigate = Route.useNavigate();
  return (
    <InviteFallbackPage
      authenticated={Boolean(session)}
      email={
        typeof session?.identity?.traits?.email === "string"
          ? session.identity.traits.email
          : undefined
      }
      continueTo={(href) => void navigate({ to: "/", href, replace: true })}
    />
  );
}

function InvitePendingRoute() {
  const { session } = Route.useRouteContext();
  return <InviteRouteSkeleton authenticated={Boolean(session)} />;
}
