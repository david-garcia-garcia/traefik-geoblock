"""Copy committed seeds into the nginx docroot and zip the lite BIN."""

from __future__ import annotations

import shutil
import zipfile
from pathlib import Path

SEEDS = Path("/seeds")
DOCROOT = Path("/www")
LITE_BIN = "IP2LOCATION-LITE-DB1.IPV6.BIN"


def main() -> None:
    DOCROOT.mkdir(parents=True, exist_ok=True)
    for item in SEEDS.iterdir():
        if item.is_file():
            shutil.copy2(item, DOCROOT / item.name)
    lite = SEEDS / LITE_BIN
    if not lite.is_file():
        raise SystemExit(f"missing {lite}")
    with zipfile.ZipFile(DOCROOT / f"{LITE_BIN}.ZIP", "w", zipfile.ZIP_STORED) as archive:
        archive.write(lite, LITE_BIN)


if __name__ == "__main__":
    main()
