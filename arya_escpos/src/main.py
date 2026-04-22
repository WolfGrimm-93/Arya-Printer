"""
Main entry point for Arya ESCPOS Service
"""
import sys
import os
from pathlib import Path

# Add src/ to Python path so that internal packages (utils, server, etc.)
# can be imported without installing the project as a package.
_src_dir = str(Path(__file__).parent)
if _src_dir not in sys.path:
    sys.path.insert(0, _src_dir)

# Detect if running as PyInstaller bundle
if getattr(sys, 'frozen', False) and hasattr(sys, '_MEIPASS'):
    bundle_dir = Path(sys._MEIPASS)
    # Install dir is one level up from the bundle (MEIPASS is _internal/)
    install_dir = Path(sys.executable).parent
else:
    bundle_dir = Path(__file__).parent.parent
    install_dir = bundle_dir

# Add libs/ to PATH for libusb-1.0.dll (Windows USB support)
if sys.platform == 'win32':
    libs_path = bundle_dir / 'libs'
    if libs_path.exists():
        os.environ['PATH'] = str(libs_path) + os.pathsep + os.environ.get('PATH', '')

from utils import init_logger, init_config, get_logger, get_config
from server.api_server import create_app


# SSL directory lives next to the executable, not inside the bundle
SSL_DIR = install_dir / "ssl"


def main():
    """Main application entry point"""

    # ── SSL setup commands (run and exit, no server needed) ───────────────
    if "--setup-ssl" in sys.argv:
        from utils.ssl_setup import run_setup
        sys.exit(run_setup(SSL_DIR, install_dir))

    if "--remove-ssl" in sys.argv:
        from utils.ssl_setup import run_remove
        sys.exit(run_remove(SSL_DIR, install_dir))

    # 1. Load configuration
    try:
        config_path = bundle_dir / "config" / "settings.yaml"
        config = init_config(str(config_path))
        print("Configuration loaded")
    except Exception as e:
        print(f"Failed to load configuration: {e}")
        return 1

    # 2. Initialize logger
    try:
        init_logger(
            level=config.logging.level,
            log_dir=config.logging.log_dir,
            retention_days=config.logging.retention_days,
            format_string=config.logging.format,
        )
        logger = get_logger()
        logger.info("=" * 60)
        logger.info("Arya ESCPOS Service Starting")
        logger.info("=" * 60)
    except Exception as e:
        print(f"Failed to initialize logger: {e}")
        return 1

    # 3. Create FastAPI app
    try:
        app = create_app()
        logger.info("FastAPI application created")
    except Exception as e:
        logger.error(f"Failed to create application: {e}")
        return 1

    # 4. Detect SSL
    from utils.ssl_setup import ssl_files_exist
    ssl_enabled = ssl_files_exist(SSL_DIR)
    ssl_kwargs = {}
    if ssl_enabled:
        ssl_kwargs = {
            "ssl_keyfile": str(SSL_DIR / "server.key"),
            "ssl_certfile": str(SSL_DIR / "server.crt"),
        }
        protocol = "https"
        logger.info(f"SSL enabled — certificates found in {SSL_DIR}")
    else:
        protocol = "http"
        logger.info("SSL not configured — running on HTTP")

    # 5. Start server
    try:
        import uvicorn

        host = config.server.host
        port = config.server.port
        display_host = "localhost" if host == "0.0.0.0" else host

        logger.info(f"Starting server on {host}:{port}")
        logger.info(f"API docs: {protocol}://{display_host}:{port}/docs")

        uvicorn.run(
            app,
            host=host,
            port=port,
            reload=config.server.reload,
            log_level=config.logging.level.lower(),
            **ssl_kwargs,
        )
    except KeyboardInterrupt:
        logger.info("Shutting down gracefully...")
    except Exception as e:
        logger.error(f"Server error: {e}")
        return 1

    return 0


if __name__ == "__main__":
    sys.exit(main())
