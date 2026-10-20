# CLI sweep 1088 — service name charset edges (live, 2026-10-20)

One disposable free web service created (the double-hyphen case) and deleted (204); the rest were refusals. No new bug filed.

- Uppercase, non-ASCII (`ü`), leading-hyphen, and trailing-hyphen names → 400 under the sweep-990 rule (lowercase letters, digits, hyphens; no edge hyphens). An interior double hyphen (`qa--1088`) → 201: consecutive hyphens inside the name are allowed.
