# Injects the git-derived firmware version as a compile-time define.
# A release build from a tag yields "v0.x.y"; a dev build yields
# "v0.x.y-N-g<hash>[-dirty]"; building outside a git checkout falls back to
# "unknown" (config/network.h carries the same fallback for builds that skip
# this script entirely).
import subprocess

Import("env")


def git_version():
    try:
        return subprocess.check_output(
            ["git", "describe", "--tags", "--always", "--dirty"],
            cwd=env["PROJECT_DIR"],
            text=True,
            stderr=subprocess.DEVNULL,
        ).strip()
    except Exception:
        return "unknown"


env.Append(CPPDEFINES=[("WISP_FW_VERSION", '\\"%s\\"' % git_version())])
