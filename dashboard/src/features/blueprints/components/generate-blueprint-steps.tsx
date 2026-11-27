import type { ReactNode } from "react";
import { ExternalLink, Loader2 } from "lucide-react";
import { Card, CardContent } from "@/common/components/ui/card";
import { CopyButton } from "@/common/components/copy-button";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";

/** bex's Blueprint docs, where Render links its infrastructure-as-code page. */
const BLUEPRINT_DOCS_HREF = "https://bex.co/docs/platform/blueprints";

/** The commit snippet of Render's second step, word for word. */
const COMMIT_SNIPPET =
  'git add render.yaml\ngit commit -m "Add render.yaml"\ngit push';

/**
 * The body of Render's Create Blueprint page (`ExportBlueprint-*.js`,
 * w3/m681), shared by the page and its pending frame so the two cannot
 * drift: three numbered steps in the wide column and the Helpful Resources
 * card beside them, below them on a narrow page. The first step's sentence
 * and YAML, its buttons, and the third step's button are the caller's; the
 * rest is Render's fixed copy.
 */
export function GenerateBlueprintSteps({
  exportBody,
  exportActions,
  connectAction,
  pending = false,
}: {
  /** Step 1's body: the resource sentence and YAML, or the loading box. */
  exportBody: ReactNode;
  /** Step 1's Download render.yaml and Copy. */
  exportActions: ReactNode;
  /** Step 3's Connect a new Blueprint. */
  connectAction: ReactNode;
  /** Marks the regions the pending frame reserves. */
  pending?: boolean;
}) {
  const { t } = useTranslations();
  const region = (name: string) => ({
    "data-skeleton-region": pending ? name : undefined,
  });
  return (
    <div className="grid grid-cols-1 gap-12 @3xl:grid-cols-6">
      <ol className="space-y-2 @3xl:col-span-4" {...region("steps")}>
        <Step
          number={1}
          title={t("blueprints.generateStepDownload")}
          region={region("export")}
          description={exportBody}
        >
          {exportActions}
        </Step>
        <Step
          number={2}
          title={t("blueprints.generateStepCommit")}
          region={region("commit")}
          description={t("blueprints.generateStepCommitBody")}
        >
          <Snippet text={COMMIT_SNIPPET} className="max-w-md" />
        </Step>
        <Step
          number={3}
          title={t("blueprints.generateStepConnect")}
          region={region("connect")}
          last
          description={
            <>
              {t("blueprints.generateStepConnectBody")}{" "}
              <a
                href={BLUEPRINT_DOCS_HREF}
                target="_blank"
                rel="noreferrer"
                className="font-medium underline underline-offset-4"
              >
                {t("blueprints.generateSeeDocs")}
              </a>
            </>
          }
        >
          {connectAction}
        </Step>
      </ol>
      <div className="@3xl:col-span-2" {...region("helpful-resources")}>
        <Card className="gap-2 py-4">
          <CardContent className="space-y-2 px-4">
            <h2 className="text-sm font-medium">
              {t("blueprints.generateHelpfulResources")}
            </h2>
            <ul>
              <li>
                <a
                  href={BLUEPRINT_DOCS_HREF}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-1.5 text-sm font-medium hover:underline"
                >
                  {t("blueprints.generateLearnBlueprints")}
                  <ExternalLink aria-hidden="true" className="size-3.5" />
                </a>
              </li>
            </ul>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

/**
 * One numbered step: Render's vertical stepper, the number in a ring with a
 * rule down to the next step, then the title, the description and the step's
 * controls.
 */
function Step({
  number,
  title,
  description,
  children,
  region,
  last = false,
}: {
  number: number;
  title: string;
  description: ReactNode;
  children?: ReactNode;
  region: { "data-skeleton-region": string | undefined };
  last?: boolean;
}) {
  return (
    <li className="grid grid-cols-[1.75rem_minmax(0,1fr)] gap-x-4" {...region}>
      <div className="flex flex-col items-center">
        <span
          aria-hidden="true"
          className="flex size-7 shrink-0 items-center justify-center rounded-full border text-sm font-medium tabular-nums"
        >
          {number}
        </span>
        {last ? null : (
          <span aria-hidden="true" className="mt-2 w-px flex-1 bg-border" />
        )}
      </div>
      <div className={cn("min-w-0 space-y-4", last ? "pb-2" : "pb-8")}>
        <h2 className="flex min-h-7 items-center font-semibold">{title}</h2>
        <div className="space-y-4 text-sm">{description}</div>
        {children}
      </div>
    </li>
  );
}

/** A code snippet with a copy control in its corner, as Render's has. */
export function Snippet({
  text,
  className,
}: {
  text: string;
  className?: string;
}) {
  const { t } = useTranslations();
  return (
    <div className={cn("relative rounded-md border bg-muted/50", className)}>
      <pre className="overflow-x-auto p-3 pr-12 font-mono text-xs leading-5 whitespace-pre-wrap break-words">
        {text}
      </pre>
      <div className="absolute top-1.5 right-1.5">
        <CopyButton
          value={text}
          label={t("common.copyCode")}
          successText={t("blueprints.copied")}
          errorText={t("blueprints.generateCopyFailed")}
        />
      </div>
    </div>
  );
}

/** Step 1 while the file is generated: Render's loading box, as tall as it. */
export function GeneratingBox() {
  const { t } = useTranslations();
  return (
    <div className="flex h-[8.75rem] items-center justify-center gap-2 rounded-md bg-muted/50 text-muted-foreground">
      <Loader2 aria-hidden="true" className="size-4 animate-spin" />
      {t("blueprints.generating")}
    </div>
  );
}
