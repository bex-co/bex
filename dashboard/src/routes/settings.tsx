import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import SettingsPage from "@/features/auth/pages/settings-page";

export const Route = createFileRoute("/settings")({
  component: SettingsPage,
  beforeLoad: requireAuth("/settings"),
  head: () => ({
    meta: [{ title: "Settings — bex" }],
  }),
});
