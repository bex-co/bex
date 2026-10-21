import { useEffect, useState } from "react";
import {
  resourceSlug,
  shortResourceSlug,
} from "@/common/lib/utils/resource-slug";

/**
 * The resource name bex-api will derive from `name` (w5/m153), or `null` for
 * an empty name and while a long name's sha256 suffix is still computing.
 * Short names resolve in the same render; only a slug over 30 characters
 * waits on Web Crypto.
 */
export function useResourceSlug(name: string): string | null {
  const short = name === "" ? null : shortResourceSlug(name);
  const needsHash = name !== "" && short === null;
  const [hashed, setHashed] = useState<{ name: string; slug: string } | null>(
    null,
  );

  useEffect(() => {
    // crypto.subtle exists only in a secure context (https or localhost).
    if (!needsHash || !globalThis.crypto?.subtle) return;
    let live = true;
    void resourceSlug(name).then((slug) => {
      if (live) setHashed({ name, slug });
    });
    return () => {
      live = false;
    };
  }, [name, needsHash]);

  if (!needsHash) return short;
  return hashed?.name === name ? hashed.slug : null;
}
