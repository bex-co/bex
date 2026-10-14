# CLI local file-path bytes are lost before upload — sweep225

Six actual pinned parser/resolver cases and twelve actual published Bex / unmodified Render commands reproduce one functional upstream issue. A literal local path ends in space, tab or newline, while its trimmed neighbor exists with different harmless contents. Both clients quietly POST the neighbor's contents and exit0: six silent wrong-file commands. With no trimmed neighbor, both report file-not-found despite the intended file existing, with no POST. Plain and interior-space paths preserve contents: four controls pass.

The native responder accepts only local POST /v1/services and records the original JSON body. It returns a labeled fake service/deploy solely to let the client finish; these fake IDs are never ownership ledger entries or production targets. Only a dummy local key is used. No request is forwarded to production, and no hosted apply/runtime success is claimed. All responders and configs are removed.

The first dummy config used a string version where the client expects an integer. Its twelve config parse errors are preserved separately and excluded from every product count. The corrected replay has the outcomes above. Current official upstream source retains path trimming; three targeted anonymous issue searches returned zero matches, which is not an exhaustive census. No external issue, PR or message was sent.

- [Actual pinned resolver](pinned-resolver-results.json)
- [Twelve actual commands and unchanged local request bodies](native-local-requests.json)
- [Native driver](native-local-driver.py.txt)
- [Resolver probe](pinned-resolver-probe.go.txt)
- [Targeted upstream checks](upstream-dedupe.json)
- [Versions and limitations](metadata.json)
