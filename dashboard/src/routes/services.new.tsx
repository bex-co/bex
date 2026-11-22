import { createFileRoute, redirect } from "@tanstack/react-router";
import {
  DEFAULT_SERVICE_TYPE,
  SERVICE_TYPE_CREATE_PATHS,
  parseNewServiceSearch,
} from "@/features/services/lib/create-context";

/**
 * bex's old service create URL (`/services/new?type=…`). The page now lives
 * at Render's per-type paths (`/web/new` …, routes/_serviceCreate.tsx); a
 * saved or shared old link lands there with its Project/Environment context.
 */
export const Route = createFileRoute("/services/new")({
  validateSearch: parseNewServiceSearch,
  beforeLoad: ({ search }) => {
    throw redirect({
      to: SERVICE_TYPE_CREATE_PATHS[search.type ?? DEFAULT_SERVICE_TYPE],
      search: {
        projectId: search.projectId,
        environmentId: search.environmentId,
      },
      replace: true,
    });
  },
});
