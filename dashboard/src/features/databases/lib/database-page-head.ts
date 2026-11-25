import {
  routeResourceTitle,
  titleHead,
  translatedText,
  type RouteResource,
} from "@/common/lib/document-head";

interface HeadMatch {
  routeId: string;
  loaderData?: unknown;
  status?: string;
}

/**
 * Document title for one of a Postgres database's pages: the page's own name
 * first, then the database and its type, e.g. "Recovery ・ orders ・
 * Database ・ bex Dashboard" (w3/m641): Render's own title, "orders ・
 * Database", led by the page's name, the shape a service tab's title has
 * (w4/m249). The label is the sidebar's own key, so the title, the heading
 * and the open sidebar entry cannot drift apart.
 */
export function databasePageHead(labelKey: string) {
  return ({ matches }: { matches: readonly HeadMatch[] }) => {
    const layout = matches.find(
      (match) => match.routeId === "/databases/$databaseId",
    );
    return titleHead(
      routeResourceTitle(
        layout?.loaderData as RouteResource<{ name: string }> | undefined,
        (database) => [
          translatedText(labelKey),
          database.name,
          translatedText("databases.resourceType"),
        ],
      ),
      matches,
    );
  };
}
