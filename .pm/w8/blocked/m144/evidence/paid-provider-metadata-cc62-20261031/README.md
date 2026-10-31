# Post-delete provider metadata corroboration — 2026-10-31

Using the existing credential through the maintained opaque loader, two GETs confirm both protected baseline volumes still return200 with exact IDs. One complete paginated GET lists all35 current project volumes, includes both baselines, and contains no volume created between the original fixture's create intent and initial Live deployment.

This is supporting provider metadata only. The original fixture's exact former provider ID/PVC lifetime was never bound; no ownership or exact404 is invented from the time window. Kubernetes, snapshot-prefix cleanup and the original full lifecycle/billing acceptance remain unproved. The source performs only GET requests; it creates no QA session, resource, SSH connection or Secret read. Raw metadata stays private; exported aggregates and response hashes contain no credential values.
