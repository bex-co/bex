# The bex infra (management) cluster base on Hetzner: SSH key + private network +
# firewall + ONE small node running single-node k3s. CAPH gets installed on top of
# this k3s afterwards (clusterctl init — a k8s-level step, see README phase 2), and
# CAPH then provisions the app cluster. This file is just the day-0 substrate.

# SSH key — single source of truth. The CAPH overlay references it by NAME
# (sshKeys.hcloud.name = var.ssh_key_name), so app-cluster nodes reuse the same key.
resource "hcloud_ssh_key" "bex" {
  name       = var.ssh_key_name
  public_key = var.ssh_public_key
}

# Private network for the infra (management) cluster. The APP cluster gets its
# OWN CAPH-managed private network (w1/m7 t005): CAPH's HetznerCluster.hcloudNetwork
# has no field to reference an existing network — it always CREATES one named after
# the cluster (`bex`). So this infra network is named `bex-infra` to avoid colliding
# with the `bex` network CAPH provisions for the app cluster. The two clusters need
# no L3 connectivity (the infra node reaches the app cluster over its public kube-API
# LB), so separate private networks are correct, not a compromise.
resource "hcloud_network" "bex" {
  name     = "bex-infra"
  ip_range = var.network_cidr
}

resource "hcloud_network_subnet" "bex" {
  network_id   = hcloud_network.bex.id
  type         = "cloud"
  network_zone = var.network_zone
  ip_range     = var.subnet_cidr
}

# Firewall for the infra node: SSH + k3s API from allowed CIDRs, ICMP for diag.
resource "hcloud_firewall" "infra" {
  name = "bex-infra"

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

# The infra (management) cluster: a single small node running single-node k3s.
# cloud-init installs k3s; CAPH is layered on later via clusterctl (README phase 2).
resource "hcloud_server" "infra" {
  name        = "bex-infra"
  server_type = var.infra_server_type
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
    role = "infra-cluster"
    bex  = "true"
  }

  depends_on = [hcloud_network_subnet.bex]
}

resource "hcloud_firewall_attachment" "infra" {
  firewall_id = hcloud_firewall.infra.id
  server_ids  = [hcloud_server.infra.id]
}

# Firewall for the CAPH-provisioned APP-cluster nodes (w1/m7 t001). The app
# cluster's servers are created by CAPH, not Terraform, so we can't attach by
# id — we attach by LABEL SELECTOR. CAPH stamps every server it owns with
# `caph-cluster-<clusterName>=owned` (see the provider's api/v1beta1 tags.go,
# constant NameHetznerProviderOwned = "caph-cluster-"), so the selector below
# matches exactly the app cluster's nodes and nothing else. New machines the
# autoscaler adds inherit the label and are firewalled automatically.
#
# Hetzner firewalls filter only the PUBLIC interface (inbound; egress is
# unrestricted). Node-to-node Kubernetes traffic (etcd, kubelet, Cilium,
# WireGuard, …) MUST therefore ride the private network — this firewall is
# only safe once t005 has flipped `hcloudNetwork.enabled: true` in the CAPH
# overlay (nodes talk over 10.0.0.0/16, which the firewall never sees). Before
# that, nodes gossip over public IPs and this ruleset would sever the cluster.
resource "hcloud_firewall" "app" {
  name = "bex-app"

  # SSH — admin/CI only (same allowlist as the infra node).
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = var.allowed_ssh_cidrs
  }

  # kube API (6443) — admin/CI only. This is the DoD's "firewall the API": the
  # apiserver binds :6443 on every control-plane node's public IP, and without
  # this rule it is internet-exposed. kubectl-via-LB still works — the Hetzner
  # control-plane LB forwards to nodes over the private network (unfiltered).
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "6443"
    source_ips = var.allowed_ssh_cidrs
  }

  # App ingress — Traefik binds the node public IP :80/:443 (docs: traefik.yaml).
  # :80 also serves the cert-manager HTTP-01 ACME challenge. Public by design.
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "80"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "443"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  # Managed tenant Postgres — Traefik's TCP/SNI entrypoint on :5432 fronts
  # external Database URLs (docs/postgresql-management.md). Public by design.
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "5432"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  # ICMP for diagnostics (ping / path-MTU discovery).
  rule {
    direction  = "in"
    protocol   = "icmp"
    source_ips = ["0.0.0.0/0", "::/0"]
  }
}

resource "hcloud_firewall_attachment" "app" {
  firewall_id     = hcloud_firewall.app.id
  label_selectors = ["caph-cluster-${var.app_cluster_name}=owned"]
}
