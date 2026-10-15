import * as React from "react";
import { getUserAgent } from "@/common/lib/user-agent";

export const MOBILE_BREAKPOINT = 768;

const MOBILE_UA_REGEX =
  /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i;

function detectMobileFromUA(): boolean {
  return MOBILE_UA_REGEX.test(getUserAgent());
}

const MOBILE_QUERY = `(max-width: ${MOBILE_BREAKPOINT - 1}px)`;

function subscribe(onChange: () => void) {
  const mql = window.matchMedia(MOBILE_QUERY);
  mql.addEventListener("change", onChange);
  return () => mql.removeEventListener("change", onChange);
}

function getSnapshot(): boolean {
  return window.innerWidth < MOBILE_BREAKPOINT;
}

export function useIsMobile() {
  // The server can only guess from the User-Agent; hydration reuses that guess
  // so the markup matches, then re-renders with the real viewport width.
  return React.useSyncExternalStore(subscribe, getSnapshot, detectMobileFromUA);
}
