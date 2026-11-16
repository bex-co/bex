import { gaInitScript, gaMeasurementId } from "@/common/lib/google-analytics";

// GA4 for the "bex dashboard" web stream (VITE_GA_MEASUREMENT_ID, see
// dashboard/AGENTS.md). Client-side navigations are counted by the stream's
// enhanced measurement ("page changes based on browser history events"), so no
// per-route hook is needed.
const GA_ID = gaMeasurementId(import.meta.env.VITE_GA_MEASUREMENT_ID);
const GA_INIT = GA_ID ? gaInitScript(GA_ID) : null;

export function GoogleAnalytics(): React.ReactNode {
  if (!GA_ID || !GA_INIT) return null;
  return (
    <>
      <script
        async
        src={`https://www.googletagmanager.com/gtag/js?id=${GA_ID}`}
      />
      <script dangerouslySetInnerHTML={{ __html: GA_INIT }} />
    </>
  );
}
