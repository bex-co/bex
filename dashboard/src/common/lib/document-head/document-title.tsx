/** React 19 hoists fallback titles into the document head during SSR. */
export function DashboardDocumentTitle({ title }: { title: string }) {
  return <title>{title}</title>;
}
