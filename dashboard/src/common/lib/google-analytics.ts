// A GA4 measurement ID is `G-` plus uppercase alphanumerics; anything else is
// refused so the value can be interpolated into the inline script safely.
const MEASUREMENT_ID = /^G-[A-Z0-9]+$/;

export function gaMeasurementId(raw: string | undefined): string | null {
  const id = raw?.trim();
  return id && MEASUREMENT_ID.test(id) ? id : null;
}

/** The dataLayer event the stream's Tag Manager container fires its Google
 * Analytics tag on (trigger `bex_ga_ready`, dashboard/AGENTS.md § Analytics). */
export const GA_READY_EVENT = "bex_ga_ready";

/**
 * The gtag bootstrap. The container's Google Analytics tag ignores an on-page
 * `gtag('config')` and reports whatever page context exists when it fires, by
 * default the document's raw URL and title. So it fires on `GA_READY_EVENT`,
 * pushed here only after an empty page context is set and the automatic page
 * view is off (dashboard/AGENTS.md § Analytics). The router then reports each
 * page by route template.
 *
 * An automated browser (`navigator.webdriver`: the QA sweeps, Playwright)
 * marks its hits `traffic_type=internal`, which the property's Internal
 * Traffic data filter excludes from every report.
 */
export const GA_INIT_SCRIPT =
  "window.dataLayer=window.dataLayer||[];" +
  "function gtag(){dataLayer.push(arguments);}" +
  "gtag('js',new Date());" +
  "gtag('set',{send_page_view:false,page_location:location.origin+'/',page_title:'(loading)'});" +
  "if(navigator.webdriver)gtag('set',{traffic_type:'internal'});" +
  `dataLayer.push({event:'${GA_READY_EVENT}'});`;
