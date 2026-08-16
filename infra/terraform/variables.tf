variable "hcloud_token" {
  description = "Hetzner Cloud API token. Pass via TF_VAR_hcloud_token (a CI secret) — never commit it."
  type        = string
  sensitive   = true
}

variable "ssh_public_key" {
  description = "SSH public key material (e.g. 'ssh-ed25519 AAAA...'). Uploaded as `ssh_key_name`; used for the bootstrap node AND by CAPH for app-cluster nodes (single source of truth)."
  type        = string
}

variable "ssh_key_name" {
  description = "Name of the hcloud SSH key. MUST match sshKeys.hcloud.name in the CAPH overlay (infra/clusterapi/overlays/hetzner-caph)."
  type        = string
  default     = "bex"
}

variable "location" {
  description = "Hetzner location for the bootstrap cluster (fsn1, nbg1, hel1, ash, hil, sin)."
  type        = string
  default     = "fsn1"
}

variable "network_zone" {
  description = "Hetzner network zone matching the location (fsn1/nbg1/hel1 => eu-central; ash => us-east; hil => us-west; sin => ap-southeast)."
  type        = string
  default     = "eu-central"
}

variable "bootstrap_server_type" {
  description = "Server type for the bootstrap cluster node. Intel cx line — 3.5x cheaper than cpx (AMD) for identical specs in fsn1. cx23 (4GB) also works; cx33 gives headroom for cert-manager + CAPI/CAPH controllers."
  type        = string
  default     = "cx33"
}

variable "image" {
  description = "OS image for the bootstrap node."
  type        = string
  default     = "ubuntu-24.04"
}

variable "k3s_channel" {
  description = "k3s release channel (or pinned version) for the single-node management cluster."
  type        = string
  default     = "stable"
}

variable "k3s_version" {
  description = "Pinned k3s release for the single-node management bootstrap (codex-security #27). Takes precedence over the channel — the installer installs exactly this version."
  type        = string
  default     = "v1.34.9+k3s1"
}

variable "k3s_install_sha256" {
  description = "SHA-256 of install.sh at the exact k3s_version Git tag. Update this together with k3s_version; cloud-init verifies before root execution."
  type        = string
  default     = "40b487f0d8ef4f5d1bf422e7bb6228cc7789c40ecc66c5ab067d396bbee9816e"
}

variable "network_cidr" {
  description = "Private network CIDR for the bootstrap cluster (the app cluster has its own CAPH-owned network)."
  type        = string
  default     = "10.0.0.0/16"
}

variable "subnet_cidr" {
  description = "Subnet CIDR within the private network."
  type        = string
  default     = "10.0.1.0/24"
}

variable "allowed_ssh_cidrs" {
  description = "CIDRs allowed to reach SSH (22) and the k3s / kube API (6443) on the infra node. Lock to your CI egress + admin IPs in prod — the default is wide open."
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

variable "bootstrap_enabled" {
  description = "Create the disposable bootstrap k3s node (bring-up / disaster recovery only). Post-pivot the app cluster is self-managed and the desired state is NO bootstrap node — flipping this to true is the first step of the DR runbook."
  type        = bool
  default     = false
}

variable "app_network_id" {
  description = "Hetzner ID of the CAPH-managed app-cluster network. CI discovers the network named `bex`; 0 leaves the post-bootstrap edge projection absent until CAPH has created it."
  type        = number
  default     = 0
}

variable "traefik_private_ip" {
  description = "Stable private IP of the production edge Load Balancer on the CAPH-managed app network."
  type        = string
  default     = "10.10.0.7"
}
