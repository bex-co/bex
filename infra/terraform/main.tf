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
