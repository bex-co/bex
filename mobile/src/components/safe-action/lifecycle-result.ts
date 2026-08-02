import type { MobileActionRunResult } from "./safe-action-panel";

export type SafeLifecycleResult =
  | { status: "success" }
  | { status: "busy" }
  | { status: "not_allowed" }
  | {
      status: "confirmation_required";
      source: "device" | "server";
      confirmation?: string;
    }
  | { status: "timeout" }
  | { status: "error"; error: unknown };

export function mobileLifecycleResult(
  result: SafeLifecycleResult,
): MobileActionRunResult {
  switch (result.status) {
    case "success":
      return { status: "success" };
    case "busy":
    case "not_allowed":
      return { status: result.status };
    case "timeout":
      return { status: "timeout" };
    case "error":
      return { status: "error", error: result.error };
    case "confirmation_required":
      return result.source === "server" && result.confirmation
        ? {
            status: "confirmation_required",
            source: "server",
            confirmation: result.confirmation,
          }
        : { status: "not_allowed" };
  }
}
