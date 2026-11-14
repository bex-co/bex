/** True for `/assets` and any path under it. */
export function isAssetsPath(pathname: string): boolean {
  return pathname === "/assets" || pathname.startsWith("/assets/");
}
