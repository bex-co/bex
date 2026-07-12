import { createFileRoute } from "@tanstack/react-router";
import VerificationPage from "@/features/auth/pages/verification-page";

export const Route = createFileRoute("/auth/verification")({
  component: VerificationPage,
  validateSearch: (search: Record<string, unknown>) => ({
    flow: typeof search.flow === "string" ? search.flow : undefined,
  }),
  head: () => ({
    meta: [{ title: "Verify email — bex" }],
  }),
});
