import { createFileRoute } from "@tanstack/react-router";
import LogoutPage from "@/features/auth/pages/logout-page";

export const Route = createFileRoute("/auth/logout")({
  component: LogoutPage,
  head: () => ({
    meta: [{ title: "Signing out — bex" }],
  }),
});
