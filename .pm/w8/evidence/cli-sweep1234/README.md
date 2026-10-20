# CLI sweep 1234 — image-backed control during the sweep 1233 stall (live, 2026-10-20)

One free `docker.io/ealen/echo-server:0.9.2` web service (`srv-dbbgmgh9a20c7385ran0`) created with `bex services create` while sweep 1233's Node deploy was stalled. It went live in 42 s and was deleted with `bex services delete --confirm` (GET 404). The cluster was scheduling new free tenant pods normally at the time. No bug filed.
