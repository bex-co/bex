#!/bin/sh
set -eu
printf 'BEX_NATIVE_INIT_PID1_OBSERVATION_V1\n'
printf 'child-pid=%s\n' "$$"
printf 'target-stat\n'; stat -c '%d %i %s %a' /native-init
printf 'target-sha256\n'; sha256sum /native-init
printf 'target-elf-header\n'; head -c 64 /native-init | od -An -v -tx1
printf 'pid1-exe-link\n'; readlink /proc/1/exe
printf 'pid1-exe-stat\n'; stat -Lc '%d %i %s %a' /proc/1/exe
printf 'pid1-exe-sha256\n'; sha256sum /proc/1/exe
printf 'pid1-exe-elf-header\n'; head -c 64 /proc/1/exe | od -An -v -tx1
printf 'pid1-cmdline-base64\n'; base64 /proc/1/cmdline
printf 'pid1-maps-BEGIN\n'; cat /proc/1/maps; printf 'pid1-maps-END\n'
printf 'pid1-map-files-BEGIN\n'
for entry in /proc/1/map_files/*; do
  target=$(readlink "$entry" 2>&1) || { printf 'map-readlink-unproved %s %s\n' "$entry" "$target"; continue; }
  case "$target" in /native-init|/native-init\ \(deleted\))
    printf 'map-entry %s %s\n' "$entry" "$target"
    stat -Lc 'map-stat %d %i %s %a' "$entry" || printf 'map-stat-unproved\n'
    ;;
  esac
done
printf 'pid1-map-files-END\n'
