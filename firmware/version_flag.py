# Injects the git-derived firmware version as a compile-time define.
# A release build from a tag yields "v0.x.y"; a dev build yields
# "v0.x.y-N-g<hash>[-dirty]"; building outside a git checkout falls back to
# "unknown" (config/network.h carries the same fallback for builds that skip
# this script entirely).
import re
import subprocess

Import("env")


def git_version():
    try:
        raw = subprocess.check_output(
            ["git", "describe", "--tags", "--always", "--dirty"],
            cwd=env["PROJECT_DIR"],
            text=True,
            stderr=subprocess.DEVNULL,
        ).strip()
    except Exception:
        return "unknown"
    # Tag names may legally contain quotes, dollar signs and angle brackets —
    # any of which would break the -D string literal or leak markup into the
    # SoftAP page. Keep only characters a version string actually needs.
    safe = re.sub(r"[^0-9A-Za-z._+-]", "_", raw)[:48]
    return safe or "unknown"


env.Append(CPPDEFINES=[("WISP_FW_VERSION", '\\"%s\\"' % git_version())])
