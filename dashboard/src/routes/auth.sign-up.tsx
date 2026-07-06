import { createFileRoute } from "@tanstack/react-router";
import RegisterPage from "@/features/auth/pages/register-page";

export const Route = createFileRoute("/auth/sign-up")({
  component: RegisterPage,
  validateSearch: (search: Record<string, unknown>) => ({
    flow: typeof search.flow === "string" ? search.flow : undefined,
  }),
  head: () => ({
    meta: [{ title: "Create your account — bex" }],
  }),
});
