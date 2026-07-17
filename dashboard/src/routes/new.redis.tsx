import { createFileRoute } from "@tanstack/react-router";
import {
  RENDER_CREATE_LANDINGS,
  redirectPreservingSuffix,
} from "@/common/lib/render-alias";

/** Render's New-menu Key Value create URL (`/new/redis` — the legacy Redis
 *  segment, live capture 2026-07-16) — the same landing as `/r/new`
 *  (`/keyvalue/new`), stated once in RENDER_CREATE_LANDINGS. */
export const Route = createFileRoute("/new/redis")({
  beforeLoad: ({ location }) => {
    redirectPreservingSuffix(RENDER_CREATE_LANDINGS.r, location);
  },
});
