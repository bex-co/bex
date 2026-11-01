"""Compare the selected exact OTP's application, NIF and header inventory."""
from pathlib import Path
import os
import stat
import sys


def complete(reference, target):
    if not reference.is_dir():
        raise ValueError("exact OTP reference inventory is missing")
    required = sorted(reference.rglob("*"))
    if not required:
        raise ValueError("exact OTP reference inventory is empty")
    missing = []
    payload = False
    for path in required:
        relative = path.relative_to(reference)
        expected = path.lstat()
        kind = stat.S_IFMT(expected.st_mode)
        if kind not in (stat.S_IFREG, stat.S_IFDIR, stat.S_IFLNK):
            raise ValueError("unsupported exact OTP reference entry: " + str(relative))
        payload |= kind != stat.S_IFDIR
        candidate = target / relative
        try:
            actual = candidate.lstat()
        except FileNotFoundError:
            missing.append(str(relative))
            continue
        if stat.S_IFMT(actual.st_mode) != kind:
            missing.append(str(relative) + " (type)")
        elif kind == stat.S_IFLNK:
            if os.readlink(candidate) != os.readlink(path):
                missing.append(str(relative) + " (link destination)")
        elif expected.st_mode & 0o111 & ~actual.st_mode:
            missing.append(str(relative) + " (executable permissions)")
    if not payload:
        raise ValueError("exact OTP reference inventory has no installed payload")
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
