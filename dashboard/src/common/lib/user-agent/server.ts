import { getRequest } from "@tanstack/react-start/server";

export function getUserAgentOnServer(): string {
  const request = getRequest();
  return request.headers.get("user-agent") ?? "";
}
