import { createFileRoute } from "@tanstack/react-router";
import ForgotPasswordPage from "@/features/auth/pages/forgot-password-page";

export const Route = createFileRoute("/auth/forgot-password")({
  component: ForgotPasswordPage,
  validateSearch: (search: Record<string, unknown>) => ({
    flow: typeof search.flow === "string" ? search.flow : undefined,
  }),
  head: () => ({
    meta: [{ title: "Forgot password — bex" }],
  }),
});
