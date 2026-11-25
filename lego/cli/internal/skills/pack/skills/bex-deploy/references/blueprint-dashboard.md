# Blueprint Dashboard fallback

Use this path when the user requests Dashboard deployment or the required programmatic operation is unavailable. Prefer the `dashboardUrl` returned by create/plan/approve for an existing Blueprint or sync; do not create a duplicate Blueprint for a manual approval.

For a new Blueprint without a returned URL:

1. Verify that the Blueprint file is committed and pushed to the intended remote branch and path. A merge into `main` is not required.
2. Normalize the Git remote to HTTPS. For example, `git@github.com:org/app.git` becomes `https://github.com/org/app`.
3. Provide `https://dashboard.bex.co/blueprint/new?repo=<URL-encoded-HTTPS-repository-URL>`.
4. Have the user complete Git-provider authorization if prompted and verify the workspace, branch, and Blueprint file path in the Dashboard. The repository deeplink alone does not select a custom branch or file path.
5. Review the proposed resources, settings, costs, and auto-sync behavior before applying. Follow [blueprints.md](blueprints.md) for manual values and the consequences of setting them after approval.
6. After the user applies the changes, follow [deployments.md](deployments.md) to verify the resulting deployments and workloads.

Describe what remains for the user and what has actually completed. A ready YAML file or a Dashboard link is not a deployed application.
