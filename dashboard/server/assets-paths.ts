/**
 * The build's asset directory (Vite `build.assetsDir`). It moved from `assets`
 * once (w4/m348): a miss under the old directory answered the HTML shell with
 * the immutable header, so browsers and Cloudflare may hold HTML under real
 * chunk URLs there for up to a year. The new directory's URLs were never
 * served that way.
 */
export const ASSETS_DIR = "_assets";

// The retired `/assets` holds nothing, so every request there is a miss.
const ASSET_DIRS = [ASSETS_DIR, "assets"];

/** True for an asset directory (current or retired) and any path under it. */
export function isAssetsPath(pathname: string): boolean {
  return ASSET_DIRS.some(
    (dir) => pathname === `/${dir}` || pathname.startsWith(`/${dir}/`),
  );
}
