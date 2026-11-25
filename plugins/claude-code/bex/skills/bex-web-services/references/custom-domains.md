# Custom domains on bex Web Services

Custom domains let clients reach your Web Service on your own hostnames while bex terminates TLS and proxies to your app.

## Adding a domain (Dashboard)

1. Open the **bex dashboard** → select the **Web Service**.
2. Go to **Settings** → **Custom Domains**.
3. Add the hostname (e.g. `www.example.com` or `api.example.com`).
4. Follow the shown **DNS** and **verification** instructions.

## DNS: CNAME to onbex.co

- For a **subdomain**, create a **CNAME** record pointing to your service’s bex hostname: **`[service-name].onbex.co`** (copy the exact target from the Dashboard).
- **Propagation** can take minutes to hours depending on TTL and provider caching.

## Apex (root) domains

Plain **CNAME at the zone apex** (`example.com`) is **not supported** by DNS RFCs at many providers.

- Use provider features such as **ALIAS**, **ANAME**, or **CNAME flattening** that synthesize A/AAAA records from a CNAME-like target, **or** use a **redirect** subdomain (e.g. `www`) as the canonical hostname.
- Follow the bex dashboard guidance for the exact record type your provider needs.

## Wildcard domains

- You can add **wildcard** hostnames such as **`*.example.com`** when your plan and workflow support it.
- DNS must include the matching **wildcard** record per your provider’s rules; TLS certificates are provisioned after **verification**.

## TLS certificates

- bex **auto-provisions** certificates for verified custom domains.
- Certificates **renew automatically**; avoid serving your own cert on the public listener for standard Web Services (TLS is at the edge).

## Verification

- bex verifies **control** of the domain via **DNS** (and/or instructions shown in the UI).
- Until verification succeeds, the domain may not serve traffic or may show certificate warnings.

## Multiple domains

- **Several hostnames** can point at the **same** Web Service; add each in **Custom Domains** or list them in a Blueprint **`domains`** field as a **list of strings** (hostnames).

Example Blueprint fragment:

```yaml
domains:
  - www.example.com
  - api.example.com
```

## Troubleshooting

| Issue | What to check |
|--------|----------------|
| Domain does not resolve | CNAME/ALIAS target typos; **TTL**; old records still cached |
| Certificate pending or invalid | Verification not complete; **conflicting** A/AAAA/CNAME records |
| Wrong site | CNAME points to a **different** `[service-name].onbex.co` |
| Apex not working | Provider lacks **flattening**; use **www** or ALIAS/ANAME |
