import { useTranslations } from "@/common/hooks/use-translations";

/**
 * A button under a resource page's state (`resource-not-found.tsx`, w3/m706):
 * a label and the dashboard path it opens.
 */
export interface ResourceStateLink {
  label: string;
  to: string;
  params?: Record<string, string>;
}

/** Render's Return home: the dashboard's home page. */
export function useReturnHomeLink(): ResourceStateLink {
  const { t } = useTranslations();
  return { label: t("common.returnHome"), to: "/" };
}

/**
 * Render's link list for a missing resource (`ResourceNotFound`'s `r()`): the
 * caller's own links, then Go to project when the cached resource sits in a
 * project, then Return home.
 */
export function useResourceNotFoundLinks(
  projectId: string | null | undefined,
  first: ResourceStateLink[] = [],
): ResourceStateLink[] {
  const { t } = useTranslations();
  const home = useReturnHomeLink();
  return [
    ...first,
    ...(projectId
      ? [
          {
            label: t("common.goToProject"),
            to: "/project/$projectId",
            params: { projectId },
          },
        ]
      : []),
    home,
  ];
}
