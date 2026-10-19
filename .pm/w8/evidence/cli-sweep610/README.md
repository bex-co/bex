# CLI sweep 610 — blueprint `projects:`/`environments:` validate dialects

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `1a49cb806`, 2026-10-19. Validate-only probes in temp dirs (removed); nothing created.

- **Environment entries are full service definitions, not name references:** listing a service by name alone under `projects[].environments[].services[]` fails with `missing properties 'type', 'ipAllowList'` at the exact path. With the full shape (`type`, `name`, `runtime`, commands, `ipAllowList`), the manifest validates and the contained service appears in the plan.
- **Environment-less projects are refused by design:** a project with no `environments` key fails with `a project must define at least one environment` — the `BLUEPRINT_PROJECT_ENVIRONMENTS_REQUIRED` guard in [blueprint_compiler.go](/lego/backend/internal/apps/blueprint_compiler.go:254), kept deliberately after Render's schema relaxed the requirement (ADR049: bex materializes projects from their environments, so an environment-less project would apply as a silent no-op).
- **Project/environment names are free-form labels:** `QA610_Proj!` as a project name and `QA610_Bad!` as an environment name both validate — projects are not resource-name addressed (same stance as env-group names, sweep 585). Two projects may declare the same environment name (`production`) without complaint.
- **Plan summary does not itemize projects:** even a fully valid project+environment manifest reports only its services in `plan.services` with `totalActions` counting services/groups/datastores. Whether the apply path materializes the project is outside what `validate` shows — see Unrun.
- Unknown project fields are refused (`additional properties 'bogus' not allowed`), and the environments-required check fires alongside schema errors rather than masking them.

## Unrun

- Blueprint **apply** of a projects manifest: does the created project/environment exist afterward, and do environment services land in it? (Apply is not exposed through the pinned CLI; would need the dashboard/API path with full cleanup.)
- A service declared both top-level and inside an environment.
