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
    # Running as PyInstaller executable
    bundle_dir = Path(sys._MEIPASS)
else:
    # Running as normal Python script
    bundle_dir = Path(__file__).parent.parent

# Add libs/ to PATH for libusb-1.0.dll (Windows USB support)
if sys.platform == 'win32':
    libs_path = bundle_dir / 'libs'
    if libs_path.exists():
        os.environ['PATH'] = str(libs_path) + os.pathsep + os.environ.get('PATH', '')

from utils import init_logger, init_config, get_logger, get_config
from server.api_server import create_app


def main():
    """Main application entry point"""
    
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
    
    # 4. Start server
    try:
        import uvicorn
        
        logger.info(f"Starting server on {config.server.host}:{config.server.port}")
        logger.info(f"API docs: http://{config.server.host if config.server.host != '0.0.0.0' else 'localhost'}:{config.server.port}/docs")
        
        uvicorn.run(
            app,
            host=config.server.host,
            port=config.server.port,
            reload=config.server.reload,
            log_level=config.logging.level.lower(),
        )
    except KeyboardInterrupt:
        logger.info("Shutting down gracefully...")
    except Exception as e:
        logger.error(f"Server error: {e}")
        return 1
    
    return 0


if __name__ == "__main__":
    sys.exit(main())
