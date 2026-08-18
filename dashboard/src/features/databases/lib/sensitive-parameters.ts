// Client mirror of bex-api's statement-logging parameter allow-list
// (`sensitiveLoggingParameters` + `sensitiveLoggingParameterPrefixes` in
// lego/backend/internal/postgres/service.go). Asserting any of these makes the
// Database parameter update can_create rather than can_operate (docs/ADR024,
// docs/ADR068 round-13 #5's lineage), so the dashboard disables Save with a role
// reason when a member without can_create sets one — matching what the server
// would refuse (w9/m84). Keep in sync with the backend; it is the source of
// truth and validates independently, so drift here only softens a UI hint.
const SENSITIVE_LOGGING_PARAMETERS = new Set([
  "log_statement",
  "log_min_duration_statement",
  "log_min_error_statement",
  "log_min_duration_sample",
  "log_statement_sample_rate",
  "log_parameter_max_length",
  "log_parameter_max_length_on_error",
]);

const SENSITIVE_LOGGING_PARAMETER_PREFIXES = ["debug_print_", "auto_explain."];

/** Whether any of the given parameter names is a statement-logging setting. */
export function setsSensitiveLoggingParameter(names: readonly string[]): boolean {
  return names.some((raw) => {
    const name = raw.trim().toLowerCase();
    if (SENSITIVE_LOGGING_PARAMETERS.has(name)) return true;
    return SENSITIVE_LOGGING_PARAMETER_PREFIXES.some((prefix) =>
      name.startsWith(prefix),
    );
  });
}
