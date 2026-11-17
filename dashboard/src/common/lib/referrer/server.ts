import { getRequest } from "@tanstack/react-start/server";

export function getReferrerOnServer(): string {
  return getRequest().headers.get("referer") ?? "";
}
