# The bex BOOTSTRAP cluster base on Hetzner: SSH key + private network +
# firewall + ONE small node running single-node k3s. CAPH gets installed on top of
# this k3s afterwards (clusterctl init — a k8s-level step, see README phase 2), and
# CAPH then provisions the app cluster. This file is just the day-0 substrate.

# SSH key — single source of truth. The CAPH overlay references it by NAME
# (sshKeys.hcloud.name = var.ssh_key_name), so app-cluster nodes reuse the same key.
resource "hcloud_ssh_key" "bex" {
  name       = var.ssh_key_name
  public_key = var.ssh_public_key
}

# Private network for the bootstrap cluster (bring-up / DR only). The APP cluster gets its
# OWN CAPH-managed private network (w1/m7 t005): CAPH's HetznerCluster.hcloudNetwork
# has no field to reference an existing network — it always CREATES one named after
# the cluster (`bex`). So this bootstrap network is named `bex-bootstrap` to avoid colliding
# with the `bex` network CAPH provisions for the app cluster. The two clusters need
# no L3 connectivity (the bootstrap node reaches the app cluster over its public kube-API
# LB), so separate private networks are correct, not a compromise.
resource "hcloud_network" "bex" {
  name     = "bex-bootstrap"
  ip_range = var.network_cidr
}

resource "hcloud_network_subnet" "bex" {
  network_id   = hcloud_network.bex.id
  type         = "cloud"
  network_zone = var.network_zone
  ip_range     = var.subnet_cidr
}

# Firewall for the bootstrap node: SSH + k3s API from allowed CIDRs, ICMP for diag.
resource "hcloud_firewall" "bootstrap" {
  name = "bex-bootstrap"

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = var.allowed_ssh_cidrs
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "6443" # k3s / kube API
    source_ips = var.allowed_ssh_cidrs
  }

  rule {
    direction  = "in"
    protocol   = "icmp"
    source_ips = ["0.0.0.0/0", "::/0"]
  }
}

# The bootstrap cluster: a single small k3s node. DISPOSABLE — it exists only
# during initial bring-up or disaster recovery; after `clusterctl move` the app
# cluster manages itself and this server is destroyed (definition retained).
# cloud-init installs k3s; CAPH is layered on later via clusterctl (README phase 2).
resource "hcloud_server" "bootstrap" {
  count = var.bootstrap_enabled ? 1 : 0 # post-pivot desired state is NO bootstrap node

  name        = "bex-bootstrap"
  server_type = var.bootstrap_server_type
  image       = var.image
  location    = var.location
  ssh_keys    = [hcloud_ssh_key.bex.id]

  user_data = templatefile("${path.module}/cloud-init.yaml.tftpl", {
    k3s_channel = var.k3s_channel
  })

  network {
    network_id = hcloud_network.bex.id
  }

  labels = {
    role = "bootstrap-cluster"
    bex  = "true"
  }

  depends_on = [hcloud_network_subnet.bex]
}

resource "hcloud_firewall_attachment" "bootstrap" {
  count = var.bootstrap_enabled ? 1 : 0

  firewall_id = hcloud_firewall.bootstrap.id
  server_ids  = hcloud_server.bootstrap[*].id
}

# App-cluster node firewall (w1/m19 t005, docs/rearchitecture.md decision 4).
# NOT the static source-IP allowlist rejected 2026-07-09 (.pm/DO_NOT_DO.md) —
# that restricted WHO may reach :22/:6443 and broke on dynamic operator/CI IPs.
# This one restricts WHICH PORTS exist on the nodes' PUBLIC interface, from
# anywhere: with the CAPH-owned private network (rearchitecture decision 1),
# every cluster-internal port (VXLAN :8472/udp — unauthenticated!, kubelet
# :10250, etcd :2379, NodePorts) moves to the private net, and the LBs reach
# nodes privately too — so public ingress needs exactly SSH (key-only,
# authentication-only baseline) and ICMP. 80/443/6443 terminate on the LBs'
# public fronts, not on nodes. Applied by LABEL SELECTOR so machines the
# autoscaler creates inherit it automatically; Hetzner firewalls don't touch
# private-net traffic at all, so east-west is unaffected by design.
resource "hcloud_firewall" "app_nodes" {
  name = "bex-app-nodes"

  apply_to {
    label_selector = "caph-cluster-bex"
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  rule {
    direction  = "in"
    protocol   = "icmp"
    source_ips = ["0.0.0.0/0", "::/0"]
  }
}

# Stable production edge (w1/m41). Terraform owns only the durable Load
# Balancer object; the hcloud CCM owns its cluster-dependent private-network
# attachment, node targets, listener services, and hcloud-ccm/service-uid label.
# Omitting those computed fields here prevents Terraform and the CCM from
# fighting over them.
#
# hcloud CCM v1.33.0 falls back from its Service UID lookup to this name, so a
# replacement Service or cluster adopts this object. Its deletion path also
# explicitly leaves a delete-protected Load Balancer in place. The lifecycle
# rule independently prevents an accidental Terraform destroy. See the live
# adoption/rebuild runbook in docs/ADR002-architecture.md.
resource "hcloud_load_balancer" "traefik" {
  name               = "bex-traefik"
  load_balancer_type = "lb11"
  location           = var.location
  delete_protection  = true

  lifecycle {
    prevent_destroy = true
    # labels belong to the CCM (hcloud-ccm/service-uid, above) — without this,
    # every apply plans to strip them back to Terraform's empty map (w10/m6).
    ignore_changes = [labels]
  }
}

# w1 rename (2026-07-11): bex-infra -> bex-bootstrap ("infra" suggested permanence;
# the node is disposable scaffolding — CAPI's own term is "bootstrap cluster").
moved {
  from = hcloud_firewall.infra
  to   = hcloud_firewall.bootstrap
}

moved {
  from = hcloud_server.infra
  to   = hcloud_server.bootstrap[0]
}

moved {
  from = hcloud_firewall_attachment.infra
  to   = hcloud_firewall_attachment.bootstrap[0]
}
