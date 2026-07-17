import { createFileRoute } from "@tanstack/react-router";
import {
  RENDER_CREATE_LANDINGS,
  redirectPreservingSuffix,
} from "@/common/lib/render-alias";

/** Render's New-menu Postgres create URL (`/new/database`, live capture
 *  2026-07-16) — the same landing as `/d/new` (the overview's URL-owned
 *  create dialog), stated once in RENDER_CREATE_LANDINGS. */
export const Route = createFileRoute("/new/database")({
  beforeLoad: ({ location }) => {
    redirectPreservingSuffix(RENDER_CREATE_LANDINGS.d, location);
  },
});
