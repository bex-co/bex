# CLI sweep 1064 — deploy-hook secret stays out of the service read (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- The service GET carries no deploy-hook field at all: the hook URL is a credential and is not embedded in the general read model (consistent with the secret-handling discipline in sweeps 964/968/1048; hook values are never printed by this loop).
