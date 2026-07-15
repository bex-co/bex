# bex worker node image (w1/m36 t001) — a Hetzner Cloud snapshot that carries
# runc, containerd, and kubelet/kubeadm/kubectl preinstalled at the versions
# kubeadm expects, so an autoscaled worker joins WITHOUT downloading ~150MB from
# github.com + pkgs.k8s.io in the scale-up hot path (docs/ADR002-architecture.md,
# FUTURE-MAYBE "Node bring-up efficiency").
#
# The versions and download/verify steps below are byte-identical to what the
# worker KubeadmConfigTemplates used to run at boot (see the git history of
# infra/clusterapi/overlays/hetzner-caph/cluster.yaml) — this only MOVES them
# from every node's first boot to a one-time bake.
#
# Runtime CONFIG stays in cloud-init (the KubeadmConfigTemplate `files:` +
# `preKubeadmCommands`): containerd.service, /etc/containerd/config.toml, the zot
# certs.d, sysctls, and `systemctl enable kubelet`. The image is deliberately
# just binaries + packages, so ONE snapshot serves both worker pools (tenant +
# platform) and the pool-specific config (e.g. the zot registry, tenant-only)
# lives where it belongs.
#
# Run it in CI (.github/workflows/snapshot.yml, workflow_dispatch) — never a
# laptop; the HCLOUD_TOKEN is a CI secret. Reproduce at the next k8s bump by
# bumping the versions below (and cluster.yaml's `version:` + the KCP images) and
# re-running the workflow, then rotate the HCloudMachineTemplate names.
#
# Local dry check:  packer init . && packer validate .
# Local build:      HCLOUD_TOKEN=… packer build .

packer {
  required_plugins {
    hcloud = {
      source  = "github.com/hetznercloud/hcloud"
      version = ">= 1.6.0"
    }
  }
}

variable "hcloud_token" {
  type      = string
  sensitive = true
  default   = env("HCLOUD_TOKEN")
}

# Pins — keep in lockstep with cluster.yaml (KCP/MachineDeployment `version:`) and
# the CONTAINERD/RUNC the KubeadmConfigTemplates used before the bake.
variable "kubernetes_version" {
  type    = string
  default = "1.31.0"
}

variable "containerd_version" {
  type    = string
  default = "1.7.26"
}

variable "runc_version" {
  type    = string
  default = "1.2.5"
}

# The base Hetzner system image the snapshot is built from — the same image the
# HCloudMachineTemplates used to boot directly (control-plane still does).
variable "base_image" {
  type    = string
  default = "ubuntu-24.04"
}

# Cheap amd64 server just for baking (prod workers are cx33, also amd64, so the
# snapshot is arch-compatible). fsn1 keeps the snapshot in the prod region.
variable "server_type" {
  type    = string
  default = "cx22"
}

variable "location" {
  type    = string
  default = "fsn1"
}

# CAPH resolves HCloudMachineTemplate.imageName against this label
# (`caph-image-name`), NOT the snapshot description — so cluster.yaml's
# `imageName: bex-worker` matches any snapshot carrying this label, newest wins.
variable "image_name" {
  type    = string
  default = "bex-worker"
}

source "hcloud" "worker" {
  token         = var.hcloud_token
  image         = var.base_image
  location      = var.location
  server_type   = var.server_type
  ssh_username  = "root"
  snapshot_name = "${var.image_name}-k8s-${var.kubernetes_version}"
  snapshot_labels = {
    caph-image-name = var.image_name
    "bex.co/role"   = "worker"
    "bex.co/k8s"    = var.kubernetes_version
  }
}

build {
  sources = ["source.hcloud.worker"]

  provisioner "shell" {
    environment_vars = [
      "DEBIAN_FRONTEND=noninteractive",
      "CONTAINERD=${var.containerd_version}",
      "RUNC=${var.runc_version}",
      "KUBERNETES_VERSION=${var.kubernetes_version}",
    ]
    inline = [
      "set -euxo pipefail",
      "TRIMMED_KUBERNETES_VERSION=$(echo $KUBERNETES_VERSION | awk -F . '{print $1 \".\" $2}')",
      "ARCH=\"$(dpkg --print-architecture)\"",
      "localectl set-locale LANG=en_US.UTF-8 || true",
      "localectl set-locale LANGUAGE=en_US.UTF-8 || true",
      "apt-get update -y",
      # Trimmed apt set (w1/m36 t003): dropped the unused at/jq/unzip/mtr/
      # apt-transport-https; keep socat (kubeadm preflight) + logrotate (kubelet
      # log rotation) + wget/curl/gpg/ca-certificates (this bake's downloads).
      "apt-get -y install wget curl socat logrotate bash-completion gpg ca-certificates",
      # (swap-disable is intentionally NOT here — every node's preKubeadmCommands
      # run `sed -i '/swap/d' /etc/fstab` + `swapoff -a` at boot; kubeadm never
      # runs at bake time, so disabling swap on the throwaway bake server is a no-op.)
      # runc → /usr/local/sbin/runc (identical to the pre-bake preKubeadmCommands)
      "wget -q https://github.com/opencontainers/runc/releases/download/v$RUNC/runc.$ARCH",
      "wget -q https://github.com/opencontainers/runc/releases/download/v$RUNC/runc.sha256sum",
      "sha256sum --check --ignore-missing runc.sha256sum",
      "install runc.$ARCH /usr/local/sbin/runc",
      "rm -f runc.$ARCH runc.sha256sum",
      # containerd → /usr/local (bin)
      "wget -q https://github.com/containerd/containerd/releases/download/v$CONTAINERD/containerd-$CONTAINERD-linux-$ARCH.tar.gz",
      "wget -q https://github.com/containerd/containerd/releases/download/v$CONTAINERD/containerd-$CONTAINERD-linux-$ARCH.tar.gz.sha256sum",
      "sha256sum --check containerd-$CONTAINERD-linux-$ARCH.tar.gz.sha256sum",
      "tar -zxf containerd-$CONTAINERD-linux-$ARCH.tar.gz -C /usr/local",
      "rm -f containerd-$CONTAINERD-linux-$ARCH.tar.gz containerd-$CONTAINERD-linux-$ARCH.tar.gz.sha256sum",
      # kubelet/kubeadm/kubectl from pkgs.k8s.io, pinned + held (as the nodes did)
      "mkdir -p /etc/apt/keyrings/",
      "curl -fsSL https://pkgs.k8s.io/core:/stable:/v$TRIMMED_KUBERNETES_VERSION/deb/Release.key | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg",
      "echo \"deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v$TRIMMED_KUBERNETES_VERSION/deb/ /\" > /etc/apt/sources.list.d/kubernetes.list",
      "apt-get update",
      "apt-get install -y kubelet=\"$KUBERNETES_VERSION-*\" kubeadm=\"$KUBERNETES_VERSION-*\" kubectl=\"$KUBERNETES_VERSION-*\"",
      "apt-mark hold kubelet kubeadm kubectl",
      "apt-get -y autoremove && apt-get -y clean",
      # Re-arm cloud-init so CAPH's per-machine bootstrap runs fresh on every new
      # worker minted from this snapshot (without this the snapshot keeps the
      # bake server's cloud-init state and the KubeadmConfig never applies).
      "cloud-init clean --logs --seed || true",
      "truncate -s0 /etc/machine-id",
      "rm -f /var/lib/dbus/machine-id && ln -s /etc/machine-id /var/lib/dbus/machine-id || true",
    ]
  }
}
