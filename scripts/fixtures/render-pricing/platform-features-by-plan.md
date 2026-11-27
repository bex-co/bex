# Platform Features by Plan — Enable powerful platform capabilities with a Pro, Scale, or Enterprise plan.

> For the complete documentation index, see [llms.txt](https://render.com/llms.txt).

Each Render workspace has a [pricing plan](<https://render.com/pricing.md>) (*Hobby*, *Pro*, *Scale*, or *Enterprise*) that determines its available features and limits. Render bills your workspace monthly according to its plan and [usage](<https://render.com/docs/faq.md#what-does-render-bill-for>).

- The *Hobby* plan does not have a monthly fee, so you only pay for usage. Each *Hobby* workspace is limited to a single team member and 25 total services.

- With a *Pro* plan or higher, you can invite unlimited [team members](<https://render.com/docs/team-members.md>) to collaborate. You also gain access to powerful operational features, such as service autoscaling and environment isolation.

- *Scale* and *Enterprise* plans support managing multiple workspaces in a unified [organization](<https://render.com/docs/organizations.md>).

You can change your workspace's plan from its *Billing* page in the [Render Dashboard](https://dashboard.render.com).

## Feature categories

### Service ops

*Pro* workspaces and higher gain access to:

| Feature | Description |
| --- | --- |
| *[Autoscaling](<https://render.com/docs/scaling.md#autoscaling>)* | Automatically scale services up and down according to their memory and CPU load. |
| *[Preview environments](<https://render.com/docs/preview-environments.md>)* | Spin up an ephemeral copy of your entire production environment for safe and comprehensive integration testing. |
| *[Performance build pipeline](<https://render.com/docs/build-pipeline.md#pipeline-tiers>)* | Run builds and other pre-deploy tasks with significantly more memory and CPU. |

### Networking

*Pro* workspaces and higher gain access to:

| Feature | Description |
| --- | --- |
| *[Dedicated outbound IPs](<https://render.com/docs/dedicated-ips.md>)* | For an additional monthly fee, send outbound traffic through a set of IP addresses unique to your workspace. |
| *[Network-isolated environments](<https://render.com/docs/projects.md#blocking-cross-environment-traffic>)* | Block private network traffic from crossing the boundary of individual project environments. |
| *[Private links](<https://render.com/docs/private-link.md>)* | Securely connect your infrastructure to non-Render providers hosted on AWS. |

*Scale* workspaces and higher also gain access to:

| Feature | Description |
| --- | --- |
| *Expanded [inbound IP rules](<https://render.com/docs/inbound-ip-rules.md>)* | Configure which IP addresses can connect to your web services and static sites over the public internet. |

### Observability

*Pro* workspaces and higher gain access to:

------

##### HTTP request logs

Automatically log details for every HTTP request to your web services from the public internet.

##### Response latency metrics

Track your web service's response times with common helpful percentiles (p50, p75, p90, and p99).

##### Metrics streaming

Push service metrics to your OpenTelemetry-compatible observability provider.

##### Log stream overrides

- With a *Pro* plan, you can disable log streaming for any individual service.

- With a *Scale* or *Enterprise* plan, you can also forward any individual service's logs to a different destination.

##### Webhooks

- With a *Pro* plan, send webhook event notifications to one destination.

- With a *Scale* or *Enterprise* plan, send different sets of notifications to up to 100 destinations.

------

### Compliance and governance

*Pro* workspaces and higher gain access to:

| Feature | Description |
| --- | --- |
| *[Compliance documentation](<https://render.com/docs/certifications-compliance.md>)* | View Render's SOC 2 Type 2 report, ISO 27001 certificate, and internal security policy. Viewing these documents also requires signing an NDA. |
| *[Audit logs](<https://render.com/docs/audit-logs.md>)* | View and export a history of material actions performed by workspace members. |
| *[OIDC](<https://render.com/docs/oidc.md>)* | Authenticate Render services with AWS, Google Cloud, Anthropic, and OpenAI using OpenID Connect (OIDC). |

*Scale* workspaces and higher also gain access to:

| Feature | Description |
| --- | --- |
| *[SAML SSO and SCIM](<https://render.com/docs/saml-sso.md>)* | Integrate your identity provider for single sign-on and automated member provisioning. |
| *[Org-level audit logs](<https://render.com/docs/audit-logs.md#organization-events>)* | View and export a history of org-level events, such as SSO configuration and member management. |
| *Additional [member roles](<https://render.com/docs/team-members.md#member-roles>)* | Assign the *Contributor*, *Viewer*, and *Billing* roles for fine-grained access control. |
| *[HIPAA-enabled workspaces](<https://render.com/docs/hipaa-compliance.md>)* | Run HIPAA-compliant applications and store protected health information on access-restricted hosts. |

### Increased limits and retention

*Pro* workspaces and higher receive:

- Unlimited [workspace members](<https://render.com/docs/team-members.md>)
  - *Hobby* workspaces are limited to a single member.
- Unlimited services
  - *Hobby* workspaces are limited to 25 total services.
- Unlimited [projects](<https://render.com/docs/projects.md>) and environments
  - *Hobby* workspaces are limited to two environments per project.
- Increased number of included [custom domains](<https://render.com/docs/custom-domains.md>)
- Increased monthly included amounts of [pipeline minutes](<https://render.com/docs/build-pipeline.md#pipeline-minutes>) and [bandwidth usage](<https://render.com/docs/outbound-bandwidth.md>)
- Increased retention of past builds for [rollbacks](<https://render.com/docs/rollbacks.md>)
- Increased retention of historical [service metrics](<https://render.com/docs/service-metrics.md>) and [logs](<https://render.com/docs/logging.md>)
- Increased per-minute task provisioning for [Render Workflows](<https://render.com/docs/workflows-limits.md#compute-limits>)