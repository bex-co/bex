from pathlib import Path
from urllib.request import Request, build_opener, HTTPRedirectHandler
from urllib.error import HTTPError
from urllib.parse import urlparse, urlencode
import hashlib,json,re,time
OUT=Path(__file__).resolve().parent
ACCEPT=", ".join(("application/vnd.oci.image.index.v1+json", "application/vnd.docker.distribution.manifest.list.v2+json", "application/vnd.oci.image.manifest.v1+json", "application/vnd.docker.distribution.manifest.v2+json"))
class Redirect(HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, msg, headers, newurl):
        result = super().redirect_request(request, fp, code, msg, headers, newurl)
        if result and urlparse(newurl).netloc != urlparse(request.full_url).netloc:
            result.remove_header("Authorization")
        return result

class Registry:
    def __init__(self, host, repository, token_host):
        self.host, self.repository, self.token_host = host, repository, token_host
        self.token = None
        self.opener = build_opener(Redirect())

    def request(self, url, accept=None, bearer=True):
        headers = {"User-Agent": "bex-readonly-m144-artifact-review"}
        if accept:
            headers["Accept"] = accept
        if bearer and self.token:
            headers["Authorization"] = "Bearer " + self.token
        with self.opener.open(Request(url, headers=headers), timeout=45) as response:
            data = response.read(4 * 1024 * 1024 + 1)
            assert len(data) <= 4 * 1024 * 1024, "metadata response too large"
            return data

    def get(self, kind, digest, filename):
        assert kind in ("manifests", "blobs") and re.fullmatch(r"sha256:[0-9a-f]{64}", digest)
        url = "https://" + self.host + "/v2/" + self.repository + "/" + kind + "/" + digest
        for attempt in range(3):
            try:
                data = self.request(url, ACCEPT if kind == "manifests" else None)
                break
            except HTTPError as error:
                if error.code == 401 and not self.token:
                    fields = dict(re.findall(r'(\w+)="([^"]*)"', error.headers.get("WWW-Authenticate", "")))
                    realm = urlparse(fields["realm"])
                    assert realm.scheme == "https" and realm.netloc == self.token_host
                    query = urlencode({"service": fields["service"], "scope": "repository:" + self.repository + ":pull"})
                    token = json.loads(self.request(fields["realm"] + "?" + query, bearer=False))
                    self.token = token.get("token", token.get("access_token"))
                    assert self.token
                elif attempt == 2:
                    raise RuntimeError("registry metadata request failed: HTTP " + str(error.code)) from None
                else:
                    time.sleep(2)
            except Exception as error:
                if attempt == 2:
                    raise RuntimeError("registry metadata request failed: " + type(error).__name__) from None
                time.sleep(2)
        else:
            raise RuntimeError("metadata retry budget exhausted")
        assert "sha256:" + hashlib.sha256(data).hexdigest() == digest, "raw metadata digest mismatch"
        (OUT / filename).write_bytes(data)
        return json.loads(data)

def review(registry, digest, prefix):
    index = registry.get("manifests", digest, prefix + "-index.json")
    result = {"index": digest, "index_media_type": index["mediaType"], "platforms": {}}
    assert isinstance(index.get("manifests"), list)
    for arch in ("amd64", "arm64"):
        matches = [row for row in index["manifests"] if row.get("platform", {}).get("os") == "linux" and row["platform"].get("architecture") == arch]
        assert len(matches) == 1, "expected one runnable manifest for " + arch
        manifest_digest = matches[0]["digest"]
        manifest = registry.get("manifests", manifest_digest, prefix + "-" + arch + "-manifest.json")
        config_digest = manifest["config"]["digest"]
        config = registry.get("blobs", config_digest, prefix + "-" + arch + "-config.json")
        assert config["os"] == "linux" and config["architecture"] == arch
        result["platforms"][arch] = {
            "manifest": manifest_digest, "config": config_digest,
            "architecture": config["architecture"], "os": config["os"],
            "labels": config.get("config", {}).get("Labels", {}),
            "diff_ids": config["rootfs"]["diff_ids"],
            "layer_count": len(manifest["layers"]),
        }
    return result


SOURCE="dd64e4ff7888828faa1b547f3a637b76d7e7da91"
binding=json.loads((OUT/"factory-binding.json").read_text())
assert binding["source"]==SOURCE and binding["run"]==41184800857 and binding["job"]==124489445679 and binding["conclusion"]=="success" and binding["full_suite_gate_passed"] and binding["actual_kernel_pass"],"actual successful factory binding required"
registry=Registry("ghcr.io","bex-co/bex-operator","ghcr.io")
query=urlencode({"service":"ghcr.io","scope":"repository:bex-co/bex-operator:pull"})
registry.token=json.loads(registry.request("https://ghcr.io/token?"+query,bearer=False))["token"]
raw=registry.request("https://ghcr.io/v2/bex-co/bex-operator/manifests/hcloud-csi-"+SOURCE,ACCEPT)
digest="sha256:"+hashlib.sha256(raw).hexdigest()
assert digest==binding["digest"],"tag differs from actual factory-published immutable digest"
candidate=review(registry,digest,"candidate")
base=review(Registry("registry-1.docker.io","hetznercloud/hcloud-csi-driver","auth.docker.io"),"sha256:459090f3c173d6708dcb3d85801509b371c9b8fffd76d704f482de3099729664","official-csi")
checks={}
for arch in ("amd64","arm64"):
    image,upstream=candidate["platforms"][arch],base["platforms"][arch]
    checks[arch]={"revision_matches_source":image["labels"].get("org.opencontainers.image.revision")==SOURCE,"source_matches_repository":image["labels"].get("org.opencontainers.image.source")=="https://github.com/bex-co/bex","version_matches_derivative":image["labels"].get("org.opencontainers.image.version")=="2.22.0-bex-bounded-ext4-v1","upstream_diff_ids_are_exact_prefix":bool(upstream["diff_ids"]) and image["diff_ids"][:len(upstream["diff_ids"])]==upstream["diff_ids"]}
assert all(all(values.values()) for values in checks.values()),checks
for arch in ("amd64","arm64"):
    config=json.loads((OUT/("candidate-"+arch+"-config.json")).read_text())
    assert config["config"]["Entrypoint"]==["/bin/hcloud-csi-driver"]
receipt={"source":SOURCE,"candidate_ref":"ghcr.io/bex-co/bex-operator@"+digest,"digest":digest,"anonymous_metadata_only":True,"all_raw_metadata_sha256_verified":True,"checks":checks,"candidate":candidate,"official_base":base}
(OUT/"raw-metadata-review.json").write_text(json.dumps(receipt,indent=2)+"\n")
print(json.dumps({key:receipt[key] for key in ("source","candidate_ref","anonymous_metadata_only","all_raw_metadata_sha256_verified","checks")},indent=2))
