import { GA_INIT_SCRIPT, gaMeasurementId } from "@/common/lib/google-analytics";

// GA4 for the "bex dashboard" web stream (VITE_GA_MEASUREMENT_ID, see
// dashboard/AGENTS.md § Analytics). Unset loads nothing.
const GA_ID = gaMeasurementId(import.meta.env.VITE_GA_MEASUREMENT_ID);

export function GoogleAnalytics(): React.ReactNode {
  if (!GA_ID) return null;
  return (
    <>
      <script
        async
        src={`https://www.googletagmanager.com/gtag/js?id=${GA_ID}`}
      />
      <script dangerouslySetInnerHTML={{ __html: GA_INIT_SCRIPT }} />
    </>
  );
}
