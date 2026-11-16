import { createFileRoute, redirect } from "@tanstack/react-router";
import { AGENT_MODEL_TERMS_URL } from "@/features/legal/lib/agent-model-terms-url";

// The agent model usage terms are a section of the bex.co Terms of Service
// (ADR091 D8). This keeps links to the dashboard copy, published 2026-11-12,
// working.
export const Route = createFileRoute("/legal/agent-models")({
  beforeLoad: () => {
    throw redirect({ href: AGENT_MODEL_TERMS_URL, statusCode: 301 });
  },
});
