# ADR: OpenBao root-token rotation and Shamir re-key runbook

**Status:** accepted; operational runbook (w7/m37). Companion to [ADR013-secrets.md](ADR013-secrets.md) (the OpenBao substrate) and [ADR015-openbao-backup-restore.md](ADR015-openbao-backup-restore.md) (backup/restore). The Shamir unseal keys and root token live in `.env` and GitHub Actions secrets; both must be updated together when either operation runs.

## Context

OpenBao's bootstrap secrets have two independent rotation axes:

| Rotation | When | Scope | Disruption |
| --- | --- | --- | --- |
| **Root token** | Suspected token compromise; audit cleanup | Replaces the privileged token only; unseal keys unchanged | None (store stays unsealed) |
| **Shamir re-key** | Suspected unseal key compromise; changing the share/threshold config | Replaces all unseal keys; root token unchanged | None (store stays unsealed; only the key material changes) |

These operations are independent: rotate the root token to retire a compromised admin token without touching the unseal keys, or re-key to replace the unseal keys without rotating the token.

**Trust boundary:** whoever runs these procedures holds `.env` and the existing root token. They have full access to the tenant secret store. This is the same trust boundary as `bao-init.sh` and `scripts/gh-secrets.sh`; apply the same custody rules: never print keys or tokens to a terminal, never log them, run in a private shell session.

## 1. Root token rotation

`bao operator generate-root` creates a new root token from the existing unseal keys plus a one-time password (OTP). The OTP is generated locally and is the only secret that must be transmitted between the person generating the OTP and the person presenting the unseal keys (if those are different people). In bex's single-operator model, both steps run in the same session.

### Procedure

```sh
# 0. Point at the live cluster.
export KUBECONFIG=infra/local/bex.kubeconfig

# 1. Set up the port-forward to OpenBao (matches what bao-init.sh does internally).
kubectl -n secrets port-forward service/openbao 8200:8200 &
PF_PID=$!
trap 'kill $PF_PID 2>/dev/null' EXIT
export BAO_ADDR=http://127.0.0.1:8200

# 2. Initiate the generate-root operation.
#    The returned OTP and nonce are written to variables — never printed to stdout
#    in a form that would appear in terminal logs.
read -r otp nonce <<< "$(bao operator generate-root -init -format=json \
  | jq -r '[.otp, .nonce] | @tsv')"
echo "nonce: $nonce"  # nonce is public; OTP is not printed
```

```sh
# 3. Present all three unseal keys from .env (threshold = 3).
#    Read each key from .env via grep, never print, pass via stdin.
set -a; source .env; set +a
for key in "$BAO_UNSEAL_KEY_1" "$BAO_UNSEAL_KEY_2" "$BAO_UNSEAL_KEY_3"; do
  printf '{"key":"%s","nonce":"%s"}' "$key" "$nonce" \
    | bao operator generate-root -nonce="$nonce" -format=json -
done
```

```sh
# 4. The last call returns an encoded_token. Decode it with the OTP.
#    Capture the encoded token from the last generate-root output and decode:
encoded_token="<value from the last generate-root output>"
new_root_token="$(bao operator generate-root \
  -decode="$encoded_token" -otp="$otp" -format=json | jq -r .token)"

# Validate: the new token can list sys/mounts.
printf '%s' "$new_root_token" \
  | BAO_ADDR=http://127.0.0.1:8200 bao kv list tenants/ >/dev/null
echo "new root token validated"
```

```sh
# 5. Revoke the OLD root token (from .env) to close the prior session.
set -a; source .env; set +a
printf '%s' "$BAO_ROOT_TOKEN" \
  | bao token revoke -self - >/dev/null || true
```

```sh
# 6. Write the new token to .env (same never-print convention as bao-init.sh).
awk -F= -v v="$new_root_token" \
  'BEGIN{OFS="="} $1=="BAO_ROOT_TOKEN"{print $1,v;next}{print}' .env > .env.tmp
mv .env.tmp .env
echo "BAO_ROOT_TOKEN updated in .env"

# 7. Push to GitHub Actions secrets.
scripts/gh-secrets.sh
echo "BAO_ROOT_TOKEN pushed to GitHub Actions secrets"
```

```sh
# 8. Verify the next deploy unseals correctly (idempotent path).
bash scripts/bao-init.sh
```

---

## 2. Shamir re-key

`bao operator rekey` replaces the unseal keys without touching the root token or the tenant data. Use this when one or more unseal keys are suspected to be compromised, or when changing the share count / threshold.

> **Take a Raft snapshot first.** Re-key rewrites the master key encryption but does not touch the stored data. A snapshot before the operation is a belt-and-suspenders precaution.

```sh
scripts/operator-kubeconfig.sh ~/.kube/bex-operator.kubeconfig \
  || export KUBECONFIG=infra/local/bex.kubeconfig  # need admin for snapshot

# Snapshot before re-key.
kubectl -n secrets create job --from=cronjob/openbao-backup openbao-backup-prerekey
kubectl -n secrets wait --for=condition=complete job/openbao-backup-prerekey --timeout=5m
```

### Procedure

```sh
# 0. Port-forward and authenticate.
kubectl -n secrets port-forward service/openbao 8200:8200 &
PF_PID=$!
trap 'kill $PF_PID 2>/dev/null' EXIT
export BAO_ADDR=http://127.0.0.1:8200
set -a; source .env; set +a
```

```sh
# 1. Initiate re-key (same 5 shares / 3 threshold as the original init; adjust if needed).
nonce="$(bao operator rekey -init -key-shares=5 -key-threshold=3 -format=json | jq -r .nonce)"
echo "rekey nonce: $nonce"
```

```sh
# 2. Present the three existing unseal keys.
for key in "$BAO_UNSEAL_KEY_1" "$BAO_UNSEAL_KEY_2" "$BAO_UNSEAL_KEY_3"; do
  result="$(printf '{"key":"%s","nonce":"%s"}' "$key" "$nonce" \
    | bao operator rekey -nonce="$nonce" -format=json -)"
  complete="$(printf '%s' "$result" | jq -r '.complete')"
  [ "$complete" = "true" ] && {
    # The last key submission returns the new keys.
    new_keys="$result"
    break
  }
done
```

```sh
# 3. Write the new unseal keys to .env (never print, use awk in-place substitution).
new_key1="$(printf '%s' "$new_keys" | jq -r '.keys_base64[0]')"
new_key2="$(printf '%s' "$new_keys" | jq -r '.keys_base64[1]')"
new_key3="$(printf '%s' "$new_keys" | jq -r '.keys_base64[2]')"

for pair in "BAO_UNSEAL_KEY_1=$new_key1" "BAO_UNSEAL_KEY_2=$new_key2" "BAO_UNSEAL_KEY_3=$new_key3"; do
  name="${pair%%=*}"; val="${pair#*=}"
  awk -F= -v n="$name" -v v="$val" \
    'BEGIN{OFS="="} $1==n{print n,v;next}{print}' .env > .env.tmp
  mv .env.tmp .env
done
echo "new unseal keys written to .env"
```

```sh
# 4. Verify the new keys unseal correctly.
#    Re-seal to prove the new keys work (skip in prod unless you have a maintenance window).
#    In prod, confirm by unsealing after the next pod restart (pod restarts come back sealed).
#
# Dev verification:
#   kubectl -n secrets delete pod openbao-0
#   kubectl -n secrets wait --for=condition=Ready pod/openbao-0 --timeout=3m  # will NOT be ready (sealed)
#   set -a; source .env; set +a
#   bash scripts/bao-init.sh                           # unseals with new keys
#   kubectl -n secrets wait --for=condition=Ready pod/openbao-0 --timeout=3m  # now Ready
echo "Verify: delete a non-leader pod and re-run bash scripts/bao-init.sh to confirm new keys unseal"
```

```sh
# 5. Push new keys to GitHub Actions.
scripts/gh-secrets.sh
echo "BAO_UNSEAL_KEY_1/2/3 pushed to GitHub Actions secrets"
```

```sh
# 6. Verify end-to-end unseal with the new keys.
bash scripts/bao-init.sh
bash scripts/bao-verify.sh
```

## After either operation

Both operations require the same follow-up:

1. **`.env` is updated** — confirm with `grep BAO_ .env | wc -l` (should be 4: three keys + token).
2. **GitHub Actions secrets are updated** — `scripts/gh-secrets.sh` pushes them; verify in the repo's Settings → Secrets.
3. **Next deploy.yml run** unseals idempotently using the new material (the `bao-init.sh` step in deploy.yml reads from GitHub Actions secrets).
4. **Physical backup** — if `.env` is also backed up offline (e.g., on an encrypted USB), update that copy now.

## Handling a partially completed re-key

If the re-key initiation succeeded but the process was interrupted before all threshold keys were presented, the rekey can be cancelled:

```sh
bao operator rekey -cancel
```

This discards the pending re-key and returns to the original unseal keys. If the re-key completed (all keys were presented) but `.env` was not updated, the new keys are returned in the last `bao operator rekey` response output — recover them from that session's shell history or the terminal buffer before they scroll away. If that output is lost, use the Raft snapshot taken before re-key to restore, then start over.
