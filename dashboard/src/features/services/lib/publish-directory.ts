import type { en } from "@/i18n";

/**
 * A static site's publish directory, as a repo-backed site's operator reads it
 * (lego/operator/internal/publish/publish.go `validate`): the clone step resolves
 * `<rootDir>/<publishPath>`, so a `..` that leaves the checkout can never
 * publish. It fails the build at deploy today, in plain words (w21/050); the
 * editor and the create form say so before Save. Whether such a path should
 * also be refused by the API at save is a separate decision (w21/056).
 *
 * An image-backed static site extracts the directory from its image, where an
 * absolute path is valid, so only the control characters bex-api refuses apply.
 */
type Reason = keyof typeof en;

// path.Join semantics: resolve "." and ".." segments lexically; an absolute
// publish path is joined under the Root Directory, as the operator does.
function joinedDirectory(rootDir: string, publishPath: string): string {
  const stack: string[] = [];
  for (const segment of `${rootDir}/${publishPath}`.split("/")) {
    if (segment === "" || segment === ".") continue;
    if (segment === "..") {
      if (stack.length > 0 && stack[stack.length - 1] !== "..") stack.pop();
      else stack.push("..");
    } else stack.push(segment);
  }
  return stack.join("/");
}

export function publishDirectoryError(
  rawPublishPath: string,
  options: { rootDir?: string | null; repoBacked: boolean },
): Reason | null {
  const publishPath = rawPublishPath.trim();
  if (publishPath === "") return null; // a required blank is its own message
  if (/[\0\r\n]/.test(publishPath)) return "services.publishPathControlChar";
  if (!options.repoBacked) return null;
  const joined = joinedDirectory((options.rootDir ?? "").trim(), publishPath);
  return joined === ".." || joined.startsWith("../")
    ? "services.publishPathEscapes"
    : null;
}
