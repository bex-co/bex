-- Argo CD health for apps/DaemonSet (w6/m518). deploy.yml installs it as
-- argocd-cm's resource.customizations.health.apps_DaemonSet, which replaces
-- Argo's built-in check for every DaemonSet. So the rule below is that check,
-- line for line (gitops-engine pkg/health/health_daemonset.go, Argo CD
-- v3.4.6), with one addition. A DaemonSet that the bex operator annotates with
-- app.bex.co/new-pods-ready = "<generation>/<desiredNumberScheduled>" reads
-- Healthy where the built-in check would read Progressing on its pod counts,
-- while both numbers still match. The operator writes that value once
-- every node runs a Ready, non-terminating pod of the current template
-- (lego/operator/internal/controller/sniproxy_rollout.go). So an old SNI proxy
-- pod that still relays its clients no longer reads as a roll in progress,
-- and a new pod that never becomes Ready still does.
-- scripts/argocd-daemonset-health.test.sh checks every other case against the
-- built-in check, in the Argo CD image production runs.
local hs = {}
local status = obj.status or {}
local generation = obj.metadata.generation or 0
local observed = status.observedGeneration or 0
local desired = status.desiredNumberScheduled or 0
local updated = status.updatedNumberScheduled or 0
local available = status.numberAvailable or 0

if generation > observed then
  hs.status = "Progressing"
  hs.message = "Waiting for rollout to finish: observed daemon set generation less than desired generation"
  return hs
end
if obj.spec ~= nil and obj.spec.updateStrategy ~= nil and obj.spec.updateStrategy.type == "OnDelete" then
  hs.status = "Healthy"
  hs.message = "daemon set " .. updated .. " out of " .. desired .. " new pods have been updated"
  return hs
end
local annotations = obj.metadata.annotations or {}
if (updated < desired or available < desired) and
    annotations["app.bex.co/new-pods-ready"] == generation .. "/" .. desired then
  hs.status = "Healthy"
  hs.message = "All " .. desired .. " nodes run a Ready pod of generation " .. generation ..
    " (app.bex.co/new-pods-ready); old pods are still draining"
  return hs
end
if updated < desired then
  hs.status = "Progressing"
  hs.message = 'Waiting for daemon set "' .. obj.metadata.name .. '" rollout to finish: ' ..
    updated .. " out of " .. desired .. " new pods have been updated..."
  return hs
end
if available < desired then
  hs.status = "Progressing"
  hs.message = 'Waiting for daemon set "' .. obj.metadata.name .. '" rollout to finish: ' ..
    available .. " of " .. desired .. " updated pods are available..."
  return hs
end
hs.status = "Healthy"
return hs
