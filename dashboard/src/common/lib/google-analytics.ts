// A GA4 measurement ID is `G-` plus uppercase alphanumerics; anything else is
// refused so the value can be interpolated into the inline script safely.
const MEASUREMENT_ID = /^G-[A-Z0-9]+$/;

export function gaMeasurementId(raw: string | undefined): string | null {
  const id = raw?.trim();
  return id && MEASUREMENT_ID.test(id) ? id : null;
}

export function gaInitScript(id: string): string {
  return `window.dataLayer=window.dataLayer||[];function gtag(){dataLayer.push(arguments);}gtag('js',new Date());gtag('config','${id}');`;
}
