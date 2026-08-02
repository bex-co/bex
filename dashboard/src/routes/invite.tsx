import { createFileRoute } from "@tanstack/react-router";
import { InviteFallbackPage } from "@/features/invites/invite-fallback-page";
import { translatedTitleHead } from "@/common/lib/document-head";

export const Route = createFileRoute("/invite")({
  component: InviteRoute,
  head: ({ match }) => {
    const translated = translatedTitleHead("invites.openingTitle", match);
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
      continueTo={(to) => void navigate({ to, replace: true })}
    />
  );
}
