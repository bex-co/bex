import { createFileRoute } from "@tanstack/react-router";
import RegisterPage from "@/features/auth/pages/register-page";

export const Route = createFileRoute("/auth/sign-up")({
  component: RegisterPage,
  validateSearch: (search: Record<string, unknown>) => ({
    flow: typeof search.flow === "string" ? search.flow : undefined,
    // OAuth2 flows can register a brand-new user too (Kratos links the
    // registration flow to the Hydra request the same way as login, w4/m9).
    login_challenge:
      typeof search.login_challenge === "string"
        ? search.login_challenge
        : undefined,
  }),
  head: () => ({
    meta: [{ title: "Create your account — bex" }],
  }),
});
