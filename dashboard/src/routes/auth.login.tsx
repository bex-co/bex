import { createFileRoute } from "@tanstack/react-router";
import LoginPage from "@/features/auth/pages/login-page";

export const Route = createFileRoute("/auth/login")({
  component: LoginPage,
  validateSearch: (search: Record<string, unknown>) => ({
    next: typeof search.next === "string" ? search.next : undefined,
    flow: typeof search.flow === "string" ? search.flow : undefined,
    // Hydra OAuth2 authorization in progress (urls.login points here, w4/m9);
    // advisory — absent/stale degrades to the ordinary login page.
    login_challenge:
      typeof search.login_challenge === "string"
        ? search.login_challenge
        : undefined,
  }),
  head: () => ({
    meta: [{ title: "Sign in — bex" }],
  }),
});
