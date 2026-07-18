import { DashboardDocumentTitle } from "./document-title";
import { translatedTitle } from ".";

export default function PendingRouteTitle() {
  return <DashboardDocumentTitle title={translatedTitle("common.loading")} />;
}
