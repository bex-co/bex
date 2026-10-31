"""Compare the selected exact OTP's application, NIF and header inventory."""
from pathlib import Path
import sys


def complete(reference, target):
    if not reference.is_dir():
        raise ValueError("exact OTP reference inventory is missing")
    required = [p.relative_to(reference) for p in reference.rglob("*")
                if p.is_file() and p.suffix in (".beam", ".hrl", ".h", ".so")]
    if not required:
        raise ValueError("exact OTP reference inventory is empty")
    missing = [str(p) for p in required if not (target / p).is_file()]
    if missing:
        print("Exact OTP donor needs same-release source build:", ", ".join(missing[:10]))
    return not missing


if __name__ == "__main__":
    try:
        result = complete(Path(sys.argv[1]), Path(sys.argv[2]))
    except (ValueError, OSError) as error:
        print(error, file=sys.stderr)
        raise SystemExit(2)
    raise SystemExit(0 if result else 1)
