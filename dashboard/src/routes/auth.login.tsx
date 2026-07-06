import { createFileRoute } from "@tanstack/react-router";
import LoginPage from "@/features/auth/pages/login-page";

export const Route = createFileRoute("/auth/login")({
  component: LoginPage,
  validateSearch: (search: Record<string, unknown>) => ({
    next: typeof search.next === "string" ? search.next : undefined,
    flow: typeof search.flow === "string" ? search.flow : undefined,
  }),
  head: () => ({
    meta: [{ title: "Sign in — bex" }],
  }),
});
