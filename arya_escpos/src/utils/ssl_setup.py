"""
SSL certificate setup using mkcert for AryaESCPOS localhost HTTPS.

mkcert installs a locally-trusted CA in Windows, Chrome, Firefox and Edge
so the browser accepts the certificate without any manual prompt.

Usage (from AryaESCPOS.exe):
    AryaESCPOS.exe --setup-ssl           # Install mkcert CA + generate certs
    AryaESCPOS.exe --remove-ssl          # Remove mkcert CA + delete certs
"""
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path


# ── mkcert helpers ────────────────────────────────────────────────────────────

def _mkcert_exe(install_dir: Path) -> Path:
    """Return path to bundled mkcert.exe (lives next to AryaESCPOS.exe)."""
    return install_dir / "mkcert.exe"


def _run_mkcert(args: list, install_dir: Path, cwd: Path | None = None) -> tuple:
    """Run mkcert with given args. Returns (success: bool, output: str)."""
    mkcert = _mkcert_exe(install_dir)
    if not mkcert.exists():
        return False, f"mkcert.exe not found at {mkcert}"
    try:
        result = subprocess.run(
            [str(mkcert)] + args,
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            cwd=str(cwd) if cwd else None,
        )
        return result.returncode == 0, (result.stdout or "") + (result.stderr or "")
    except Exception as e:
        return False, str(e)


# ── trust store ───────────────────────────────────────────────────────────────

def ca_is_installed(install_dir: Path) -> bool:
    """Return True if mkcert CA is already in any Windows Root trust store."""
    try:
        for store_flag in ([], ["-user"]):
            result = subprocess.run(
                ["certutil"] + store_flag + ["-store", "Root"],
                capture_output=True,
                text=True,
                encoding="utf-8",
                errors="replace",
            )
            if result.returncode == 0 and "mkcert" in result.stdout.lower():
                return True
        return False
    except Exception:
        return False


def install_ca(install_dir: Path) -> bool:
    """Install mkcert CA in Windows, Chrome, Firefox and Edge trust stores."""
    ok, out = _run_mkcert(["-install"], install_dir)
    if ok:
        print("mkcert CA installed in Windows and browser trust stores")
    else:
        print(f"mkcert -install failed: {out.strip()}")
    return ok


def remove_ca(install_dir: Path) -> bool:
    """Remove mkcert CA from all trust stores."""
    ok, out = _run_mkcert(["-uninstall"], install_dir)
    if ok:
        print("mkcert CA removed from trust stores")
    else:
        print(f"mkcert -uninstall failed: {out.strip()}")
    return ok


# ── certificate files ─────────────────────────────────────────────────────────

def generate_certs(ssl_dir: Path, install_dir: Path) -> bool:
    """Generate server certificate for localhost using mkcert."""
    ssl_dir.mkdir(parents=True, exist_ok=True)
    ok, out = _run_mkcert(
        ["-key-file", "server.key", "-cert-file", "server.crt", "localhost", "127.0.0.1"],
        install_dir,
        cwd=ssl_dir,
    )
    if ok:
        print(f"Certificates generated in: {ssl_dir}")
    else:
        print(f"mkcert cert generation failed: {out.strip()}")
    return ok


def ssl_files_exist(ssl_dir: Path) -> bool:
    """Return True if all SSL files required by uvicorn are present."""
    return (ssl_dir / "server.crt").exists() and (ssl_dir / "server.key").exists()


def cert_days_remaining(ssl_dir: Path) -> int | None:
    """Return days until server.crt expires, or None if file doesn't exist."""
    cert_path = ssl_dir / "server.crt"
    if not cert_path.exists():
        return None
    try:
        from cryptography import x509
        cert = x509.load_pem_x509_certificate(cert_path.read_bytes())
        delta = cert.not_valid_after_utc - datetime.now(timezone.utc)
        return delta.days
    except Exception:
        # Fallback: mkcert generates 2-year certs; estimate from file age
        import time
        age_days = (time.time() - cert_path.stat().st_mtime) / 86400
        return max(0, int(730 - age_days))


def cert_needs_renewal(ssl_dir: Path, renew_threshold_days: int = 30) -> bool:
    """Return True if cert is expired, invalid, or expiring within threshold."""
    days = cert_days_remaining(ssl_dir)
    if days is None:
        return True
    return days <= renew_threshold_days


# ── main commands ─────────────────────────────────────────────────────────────

def run_setup(ssl_dir: Path, install_dir: Path | None = None) -> int:
    """Configure SSL for localhost using mkcert. Returns exit code.

    install_dir defaults to ssl_dir.parent when not provided (dev mode).

    Logic:
    - CA installed + files valid (>30 days)  -> skip (nothing to do)
    - Cert expired or <= 30 days remaining   -> remove CA, regenerate, reinstall
    - CA not installed                       -> install CA
    - Files missing                          -> generate files
    """
    if install_dir is None:
        install_dir = ssl_dir.parent

    files_ok = ssl_files_exist(ssl_dir)
    ca_ok = ca_is_installed(install_dir)
    needs_renewal = cert_needs_renewal(ssl_dir)
    days = cert_days_remaining(ssl_dir)

    if ca_ok and files_ok and not needs_renewal:
        print(f"SSL already configured — certificate valid for {days} more days.")
        print("Nothing to do.")
        return 0

    if files_ok and needs_renewal:
        if days is not None and days <= 0:
            print(f"SSL certificate EXPIRED {abs(days)} days ago — renewing...")
        else:
            print(f"SSL certificate expires in {days} days — renewing...")
        remove_ca(install_dir)
        files_ok = False
        ca_ok = False

    if not ca_ok:
        print("Installing mkcert CA in trust stores...")
        if not install_ca(install_dir):
            print("ERROR: Failed to install CA — try running as Administrator")
            return 1

    if not files_ok:
        print("Generating SSL certificates for localhost HTTPS...")
        if not generate_certs(ssl_dir, install_dir):
            print("ERROR: Failed to generate certificates")
            return 1

    print()
    print("SSL setup complete.")
    print("The service will now use: https://localhost:58181")
    return 0


def run_remove(ssl_dir: Path, install_dir: Path | None = None) -> int:
    """Remove mkcert CA and delete cert files. Returns exit code."""
    if install_dir is None:
        install_dir = ssl_dir.parent

    print("Removing mkcert CA from trust stores...")
    success = remove_ca(install_dir)

    for filename in ("server.crt", "server.key"):
        f = ssl_dir / filename
        if f.exists():
            f.unlink()
            print(f"Deleted: {f}")

    return 0 if success else 1
