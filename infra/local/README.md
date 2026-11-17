# infra/local — local bootstrap (kind infra cluster + CAPD + KWOK)

Stands up the local mock of the Hetzner substrate, entirely in Docker:

- a **kind** cluster as the **infra cluster** (Cluster API's management cluster);
- **CAPD** (Cluster API Docker provider) so "machines" are Docker-container nodes;
- a CAPD **app cluster** (`infra/clusterapi/overlays/local-capd`) where bex runs;
- (optional) **KWOK** for testing the allocator against many fake nodes cheaply.

Run `scripts/mock-cluster.sh` to bring it up. Add/remove a machine = `kubectl scale machinedeployment ... --replicas=N` against the infra cluster. Swap CAPD → CAPH (`infra/clusterapi/overlays/hetzner-caph`) for Hetzner; bex unchanged.

## Host requirement: inotify limits

Every node container runs as root on the Docker host, and the kernel's inotify limits are per user, so the mgmt node, every CAPD node and the host's own root daemons share one budget. A node past it cannot start containerd's CRI plugin (`failed to create fsnotify watcher: too many open files`): its Machine never joins, and CAPI replaces it every few minutes. The kernel default `fs.inotify.max_user_instances=128` cannot hold the cluster.

The script sizes 64 instances and 4096 watches per node container, for the mgmt node, the control plane, the platform worker and the tenant pool at its autoscaler max. The default topology therefore needs `fs.inotify.max_user_instances` ≥ 512 and `fs.inotify.max_user_watches` ≥ 32768, and `scale N` above 5 needs 64 more instances per extra node. Raise both on the Docker host (it needs root; on macOS, inside the Docker VM):

```sh
sudo sysctl -w fs.inotify.max_user_instances=1024 fs.inotify.max_user_watches=1048576
```

To keep the values across reboots, put the same two settings in `/etc/sysctl.d/`. This is kind's known issue "Pod errors due to too many open files".

`mock-cluster.sh` (bring-up, `reprovision`, `scale N`) reads both sysctls before it creates or scales a node container. Below the requirement it exits 1 within seconds, naming the limit and that command, and `reprovision` deletes nothing. When a Machine has still not joined after the startup wait, the script reads each node container's containerd journal for that fsnotify line. If it finds the line, it names the nodes and the same command. That covers hosts where the sysctls cannot be read, such as macOS.
