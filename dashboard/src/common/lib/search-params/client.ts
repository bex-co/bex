export function getSearchParamOnClient(key: string): string | null {
  const value = new URLSearchParams(window.location.search).get(key);
  return value !== null ? decodeURIComponent(value) : null;
}
