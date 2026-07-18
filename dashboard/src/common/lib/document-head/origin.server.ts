import { getRequest } from "@tanstack/react-start/server";

export function getDashboardOriginOnServer(): string | null {
  return new URL(getRequest().url).origin;
}
