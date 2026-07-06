import type { Session } from "@ory/client-fetch";
import { createFrontendApi } from "@/common/lib/ory/frontend";

export async function fetchSession(): Promise<Session | null> {
  const cookie = import.meta.env.SSR
    ? (await import("@tanstack/react-start/server")).getRequestHeader("cookie")
    : undefined;

  try {
    return await createFrontendApi(cookie).toSession();
  } catch {
    return null;
  }
}
