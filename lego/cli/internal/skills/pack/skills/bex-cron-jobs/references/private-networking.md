<!-- shared:private-networking -->
## Private networking

bex services can communicate over a private network without traversing the public internet. Private-network peers must belong to the same workspace and run in the same region. An isolated project environment can additionally block traffic both to and from resources outside that environment, so workspace and region matching alone do not guarantee connectivity.

Before designing, configuring, or troubleshooting private-network communication, fetch the current bex private-network reference:

Using the documentation-retrieval workflow in the root `SKILL.md`, retrieve and read the [Markdown reference](https://render.com/docs/private-network.md) or its [HTML version](https://render.com/docs/private-network).

Read the relevant sections. Confirm current service-type capabilities, port restrictions, environment-isolation behavior, discovery-hostname guidance, and plan requirements instead of relying on a memorized catalog.

Web services and private services have internal hostnames and can receive private-network traffic. A private service must bind at least one port; use a background worker instead when a workload does not need to accept inbound connections. bex Postgres and Key Value instances provide internal connection URLs. Workflows, background workers, and cron jobs can initiate private-network connections but do not have an internal address for receiving them. Static sites are not on the private network. Free web services can initiate private-network connections but cannot receive them.

Prefer the internal address or URL shown under **Connect > Internal** in the bex dashboard. Include the protocol when the client or server requires one; an address such as `service:10000` does not by itself establish whether to use HTTP, HTTPS, or another protocol. A process that receives traffic must listen on the intended port and on an interface reachable outside its own container, such as `0.0.0.0`, rather than only on loopback.

For Blueprint-managed connections, fetch the current Blueprint specification and use supported service or database references instead of copying a generated hostname, port, credential, or connection string. Web and private services currently expose `host`, `port`, and `hostport` properties. A Blueprint can obtain another service's discovery hostname by referencing that service's `RENDER_DISCOVERY_SERVICE` environment variable with `fromService.envVarKey`; the discovery hostname is not a service property. The fetched specification is authoritative for the syntax and supported references.

A service can expose at most 75 ports on the private network. Ports `18012`, `18013`, and `19099` are unavailable for private-network communication. Port `10000` is not generally blocked: for a web service, private traffic to port `10000` routes to its primary HTTP server even when that server binds to another port. In that case, both port `10000` and the actual primary HTTP port route to the same server.

Use a service's normal internal hostname for ordinary service-to-service traffic. For advanced per-instance communication, each web or private service has a discovery hostname, conventionally formed by appending `-discovery` to its internal hostname, that resolves to all active instance IP addresses. A service receives its own discovery hostname in `RENDER_DISCOVERY_SERVICE`. Use DNS APIs that honor the system resolver configuration, and do not persist resolved instance IPs because they can change between deploys. Prefer hostname-based communication unless per-instance addressing or custom load balancing is specifically required.

When a private-network connection fails, verify the workspace, region, project-environment isolation settings, whether the destination service type accepts inbound traffic, the internal hostname or URL, protocol, port, listening interface, current deploy health, and DNS resolution. Do not substitute a public URL or expose a private service merely to bypass a private-network configuration problem.

AWS PrivateLink is a separate Pro-or-higher feature for connecting a bex private network to compatible AWS-hosted systems. Fetch its current documentation before proposing or configuring it; do not treat ordinary bex service-to-service networking and PrivateLink as interchangeable.
<!-- /shared:private-networking -->
