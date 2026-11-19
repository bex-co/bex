import { cn } from "@/common/lib/utils/utils";

/**
 * A deploy's commit as Render shows it: the short SHA, linked to its diff on
 * the forge when the repo is browsable, plain text otherwise (an
 * image-backed service). The full SHA is the title. Shared by the Deploys
 * list rows and the deploy page's Source fact.
 */
export function CommitSha({
  commitId,
  url,
  className,
}: {
  commitId: string;
  /** The commit's diff, or null when the repo cannot be browsed. */
  url: string | null;
  className?: string;
}) {
  const short = commitId.slice(0, 7);
  if (!url) {
    return (
      <span className={cn("font-mono", className)} title={commitId}>
        {short}
      </span>
    );
  }
  return (
    <a
      href={url}
      target="_blank"
      rel="noreferrer"
      title={commitId}
      // The ::after grows the tap target to 44px on a touch layout without
      // moving the line.
      className={cn(
        "relative font-mono underline decoration-muted-foreground/60 underline-offset-2 hover:decoration-foreground max-lg:after:absolute max-lg:after:-inset-x-2 max-lg:after:-inset-y-3.5 max-lg:after:content-['']",
        className,
      )}
    >
      {short}
    </a>
  );
}
