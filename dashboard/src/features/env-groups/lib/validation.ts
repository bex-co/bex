/** bex-api and Render both accept any non-empty group display name. */
export function isValidEnvGroupName(name: string): boolean {
  return name.trim().length > 0;
}
