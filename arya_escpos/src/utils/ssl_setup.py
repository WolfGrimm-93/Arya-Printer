"""
SSL certificate setup for AryaESCPOS localhost HTTPS.

Generates a self-signed CA + server certificate for localhost,
installs the CA in the Windows trusted root store so Chrome and
other browsers accept the HTTPS connection without warnings.

Usage (from AryaESCPOS.exe):
    AryaESCPOS.exe --setup-ssl           # Generate certs + install CA
    AryaESCPOS.exe --remove-ssl          # Remove CA from Windows trust store
"""
import ipaddress
import subprocess
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path

from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.x509.oid import ExtendedKeyUsageOID, NameOID


def generate_certs(ssl_dir: Path) -> bool:
    """Generate CA + server certificate PEM files in ssl_dir.

    Files created:
        ca.crt      — Root CA certificate (installed in Windows trust store)
        server.crt  — Server certificate for localhost (used by uvicorn)
        server.key  — Server private key (used by uvicorn)
    """
    ssl_dir.mkdir(parents=True, exist_ok=True)
    now = datetime.now(timezone.utc)

    # ── CA key + certificate (10 years) ───────────────────────────────────
    ca_key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    ca_name = x509.Name([
        x509.NameAttribute(NameOID.COUNTRY_NAME, "DO"),
        x509.NameAttribute(NameOID.ORGANIZATION_NAME, "Arya Development"),
        x509.NameAttribute(NameOID.ORGANIZATIONAL_UNIT_NAME, "Arya ESCPOS Service"),
        x509.NameAttribute(NameOID.COMMON_NAME, "Arya ESCPOS Service - Root CA"),
    ])

    ca_cert = (
        x509.CertificateBuilder()
        .subject_name(ca_name)
        .issuer_name(ca_name)
        .public_key(ca_key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(now)
        .not_valid_after(now + timedelta(days=3650))
        .add_extension(x509.BasicConstraints(ca=True, path_length=None), critical=True)
        .add_extension(
            x509.SubjectKeyIdentifier.from_public_key(ca_key.public_key()),
            critical=False,
        )
        .sign(ca_key, hashes.SHA256())
    )

    # ── Server key + certificate (5 years) ────────────────────────────────
    server_key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    server_name = x509.Name([
        x509.NameAttribute(NameOID.COUNTRY_NAME, "DO"),
        x509.NameAttribute(NameOID.ORGANIZATION_NAME, "Arya Development"),
        x509.NameAttribute(NameOID.ORGANIZATIONAL_UNIT_NAME, "Arya ESCPOS Service"),
        x509.NameAttribute(NameOID.COMMON_NAME, "localhost"),
    ])

    server_cert = (
        x509.CertificateBuilder()
        .subject_name(server_name)
        .issuer_name(ca_name)
        .public_key(server_key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(now)
        .not_valid_after(now + timedelta(days=1825))
        .add_extension(
            x509.SubjectAlternativeName([
                x509.DNSName("localhost"),
                x509.IPAddress(ipaddress.ip_address("127.0.0.1")),
            ]),
            critical=False,
        )
        .add_extension(
            x509.BasicConstraints(ca=False, path_length=None),
            critical=True,
        )
        .add_extension(
            x509.ExtendedKeyUsage([ExtendedKeyUsageOID.SERVER_AUTH]),
            critical=False,
        )
        .sign(ca_key, hashes.SHA256())
    )

    # ── Write files ────────────────────────────────────────────────────────
    (ssl_dir / "ca.crt").write_bytes(
        ca_cert.public_bytes(serialization.Encoding.PEM)
    )
    (ssl_dir / "server.crt").write_bytes(
        server_cert.public_bytes(serialization.Encoding.PEM)
    )
    (ssl_dir / "server.key").write_bytes(
        server_key.private_bytes(
            encoding=serialization.Encoding.PEM,
            format=serialization.PrivateFormat.TraditionalOpenSSL,
            encryption_algorithm=serialization.NoEncryption(),
        )
    )

    print(f"Certificates generated in: {ssl_dir}")
    return True


def ca_is_installed() -> bool:
    """Return True if the AryaESCPOS CA is already in the Windows trust store."""
    try:
        result = subprocess.run(
            ["certutil", "-store", "Root", "Arya ESCPOS Service - Root CA"],
            capture_output=True,
            text=True,
        )
        return result.returncode == 0 and "Arya ESCPOS Service" in result.stdout
    except Exception:
        return False


def install_ca(ca_cert_path: Path) -> bool:
    """Install CA certificate in Windows LocalMachine\\Root trust store."""
    try:
        result = subprocess.run(
            ["certutil", "-addstore", "-f", "Root", str(ca_cert_path)],
            capture_output=True,
            text=True,
        )
        if result.returncode == 0:
            print("CA certificate installed in Windows trusted root store")
            return True
        print(f"certutil error: {result.stderr.strip()}")
        return False
    except Exception as e:
        print(f"Failed to install CA: {e}")
        return False


def remove_ca() -> bool:
    """Remove AryaESCPOS CA from Windows LocalMachine\\Root trust store."""
    try:
        result = subprocess.run(
            ["certutil", "-delstore", "Root", "Arya ESCPOS Service - Root CA"],
            capture_output=True,
            text=True,
        )
        if result.returncode == 0:
            print("CA removed from Windows trusted root store")
            return True
        print(f"certutil error: {result.stderr.strip()}")
        return False
    except Exception as e:
        print(f"Failed to remove CA: {e}")
        return False


def ssl_files_exist(ssl_dir: Path) -> bool:
    """Return True if all SSL files required by uvicorn are present."""
    return (
        (ssl_dir / "server.crt").exists()
        and (ssl_dir / "server.key").exists()
    )


def cert_days_remaining(ssl_dir: Path) -> int | None:
    """Return days until server.crt expires, or None if file doesn't exist."""
    cert_path = ssl_dir / "server.crt"
    if not cert_path.exists():
        return None
    try:
        cert = x509.load_pem_x509_certificate(cert_path.read_bytes())
        delta = cert.not_valid_after_utc - datetime.now(timezone.utc)
        return delta.days
    except Exception:
        return None


def cert_needs_renewal(ssl_dir: Path, renew_threshold_days: int = 30) -> bool:
    """Return True if cert is expired, invalid, or expiring within threshold."""
    days = cert_days_remaining(ssl_dir)
    if days is None:
        return True
    return days <= renew_threshold_days


def run_setup(ssl_dir: Path) -> int:
    """Configure SSL for localhost. Returns exit code.

    Verifica el estado actual antes de actuar:
    - CA instalado + archivos validos (>30 dias)  -> no hace nada
    - Certificado vencido o <= 30 dias restantes  -> renueva (borra CA, regenera, reinstala)
    - CA instalado + archivos faltan              -> regenera archivos, no toca CA
    - CA no instalado                             -> genera archivos e instala CA
    """
    files_ok = ssl_files_exist(ssl_dir)
    ca_ok = ca_is_installed()
    needs_renewal = cert_needs_renewal(ssl_dir)
    days = cert_days_remaining(ssl_dir)

    # Caso: todo OK y vigente
    if ca_ok and files_ok and not needs_renewal:
        print(f"SSL already configured — certificate valid for {days} more days.")
        print("Nothing to do.")
        return 0

    # Caso: certificado vencido o proximo a vencer
    if files_ok and needs_renewal:
        if days is not None and days <= 0:
            print(f"SSL certificate EXPIRED {abs(days)} days ago — renewing...")
        else:
            print(f"SSL certificate expires in {days} days — renewing...")
        remove_ca()
        files_ok = False
        ca_ok = False

    # Caso: archivos faltan o se acaban de borrar por renovacion
    if not files_ok:
        print("Generating SSL certificates for localhost HTTPS...")
        if not generate_certs(ssl_dir):
            print("ERROR: Failed to generate certificates")
            return 1

    # Caso: CA no instalado o se acaba de borrar por renovacion
    if not ca_ok:
        print("Installing CA in Windows trust store...")
        if not install_ca(ssl_dir / "ca.crt"):
            print("ERROR: Failed to install CA — try running as Administrator")
            return 1

    print()
    print("SSL setup complete.")
    print("The service will now use: https://localhost:58181")
    return 0


def run_remove(ssl_dir: Path) -> int:
    """Remove CA from Windows trust store. Returns exit code."""
    print("Removing AryaESCPOS CA from Windows trust store...")
    success = remove_ca()

    # Also delete cert files if they exist
    for filename in ("ca.crt", "server.crt", "server.key"):
        f = ssl_dir / filename
        if f.exists():
            f.unlink()
            print(f"Deleted: {f}")

    return 0 if success else 1
