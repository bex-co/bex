import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { UsagePage } from "@/features/usage/components/usage-page";

export const Route = createFileRoute("/usage")({
  component: UsagePage,
  beforeLoad: requireAuth("/usage"),
  head: () => ({
    meta: [{ title: "Usage — bex" }],
  }),
});
