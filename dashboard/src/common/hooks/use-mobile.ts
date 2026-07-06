import * as React from "react";
import { getUserAgent } from "@/common/lib/user-agent";

export const MOBILE_BREAKPOINT = 768;

const MOBILE_UA_REGEX =
  /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i;

function detectMobileFromUA(): boolean {
  return MOBILE_UA_REGEX.test(getUserAgent());
}

export function useIsMobile() {
  const [isMobile, setIsMobile] = React.useState<boolean>(detectMobileFromUA);

  React.useEffect(() => {
    const mql = window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT - 1}px)`);
    const onChange = () => {
      setIsMobile(window.innerWidth < MOBILE_BREAKPOINT);
    };
    mql.addEventListener("change", onChange);
    setIsMobile(window.innerWidth < MOBILE_BREAKPOINT);
    return () => mql.removeEventListener("change", onChange);
  }, []);

  return isMobile;
}
