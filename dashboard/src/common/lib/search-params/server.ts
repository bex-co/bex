import { getRequest } from "@tanstack/react-start/server";

export function getSearchParamOnServer(key: string): string | null {
  const request = getRequest();
  const value = new URL(request.url).searchParams.get(key);
  return value !== null ? decodeURIComponent(value) : null;
}
