"""
Script para compilar Arya ESCPOS a ejecutable standalone usando PyInstaller.
Ejecutar desde la raiz del proyecto: python build_executable.py
"""
import os
import sys
import shutil
import subprocess
from pathlib import Path

print("=" * 60)
print("ARYA ESCPOS - BUILD EXECUTABLE")
print("=" * 60)

# Configuracion
PROJECT_ROOT = Path(__file__).parent
DIST_FOLDER = PROJECT_ROOT / "dist"
BUILD_FOLDER = PROJECT_ROOT / "build"
SPEC_FILE = PROJECT_ROOT / "AryaESCPOS.spec"
MAIN_SCRIPT = PROJECT_ROOT / "src" / "main.py"
OUTPUT_NAME = "AryaESCPOS"

print(f"\nDirectorio del proyecto: {PROJECT_ROOT}")
print(f"Script principal: {MAIN_SCRIPT}")

# Verificar que existe main.py
if not MAIN_SCRIPT.exists():
    print(f"\nERROR: No se encuentra {MAIN_SCRIPT}")
    sys.exit(1)

print("Script principal encontrado")

# Limpiar builds anteriores
print("\nLimpiando builds anteriores...")
for folder in [DIST_FOLDER, BUILD_FOLDER]:
    if folder.exists():
        shutil.rmtree(folder)
        print(f"  Eliminado: {folder}")

if SPEC_FILE.exists():
    SPEC_FILE.unlink()
    print(f"  Eliminado: {SPEC_FILE}")

# Construir comando PyInstaller
print("\nCompilando con PyInstaller...")
print("Esto puede tardar varios minutos...\n")

pyinstaller_args = [
    "pyinstaller",
    "--name", OUTPUT_NAME,
    "--onedir",
    "--console",
    "--clean",

    # Paths
    "--paths", str(PROJECT_ROOT / "src"),

    # Datos necesarios
    "--add-data", f"config{os.pathsep}config",
    "--add-data", f"libs{os.pathsep}libs",

    # Hidden imports - modulos del proyecto
    "--hidden-import", "utils",
    "--hidden-import", "server",
    "--hidden-import", "server.api_server",
    "--hidden-import", "server.routes",
    "--hidden-import", "server.device_routes",
    "--hidden-import", "server.print_routes",
    "--hidden-import", "server.config_routes",
    "--hidden-import", "server.schemas",
    "--hidden-import", "server.adapter_factory",
    "--hidden-import", "server.compat",
    "--hidden-import", "adapters",
    "--hidden-import", "adapters.usb_adapter",
    "--hidden-import", "adapters.serial_adapter",
    "--hidden-import", "adapters.network_adapter",
    "--hidden-import", "adapters.bluetooth_adapter",
    "--hidden-import", "core",
    "--hidden-import", "core.base_adapter",
    "--hidden-import", "core.command_builder",
    "--hidden-import", "utils.config",
    "--hidden-import", "utils.logger",
    "--hidden-import", "utils.exceptions",
    "--hidden-import", "utils.pdf_printer",
    "--hidden-import", "utils.document_converter",

    # Hidden imports - librerias externas
    "--hidden-import", "uvicorn.logging",
    "--hidden-import", "uvicorn.loops",
    "--hidden-import", "uvicorn.loops.auto",
    "--hidden-import", "uvicorn.protocols",
    "--hidden-import", "uvicorn.protocols.http",
    "--hidden-import", "uvicorn.protocols.http.auto",
    "--hidden-import", "uvicorn.protocols.websockets",
    "--hidden-import", "uvicorn.protocols.websockets.auto",
    "--hidden-import", "uvicorn.lifespan",
    "--hidden-import", "uvicorn.lifespan.on",
    "--hidden-import", "win32timezone",
    "--hidden-import", "win32print",
    "--hidden-import", "win32api",
    "--hidden-import", "win32con",
    "--hidden-import", "pywintypes",

    # Excluir modulos innecesarios
    "--exclude-module", "sqlalchemy",
    "--exclude-module", "alembic",
    "--exclude-module", "matplotlib",
    "--exclude-module", "numpy",
    "--exclude-module", "pandas",
    "--exclude-module", "scipy",
    "--exclude-module", "tkinter",
    "--exclude-module", "pytest",
    "--exclude-module", "black",
    "--exclude-module", "flake8",
    "--exclude-module", "setuptools",
    "--exclude-module", "pkg_resources",

    # Script principal
    str(MAIN_SCRIPT)
]

try:
    result = subprocess.run(pyinstaller_args, check=True, capture_output=False)

    print("\n" + "=" * 60)
    print("COMPILACION EXITOSA!")
    print("=" * 60)

    exe_folder = DIST_FOLDER / OUTPUT_NAME
    exe_path = exe_folder / f"{OUTPUT_NAME}.exe"

    if exe_path.exists():
        # Copy mkcert.exe next to the main executable (required for SSL)
        mkcert_src = PROJECT_ROOT / "mkcert.exe"
        mkcert_dst = exe_folder / "mkcert.exe"
        if mkcert_src.exists():
            shutil.copy2(mkcert_src, mkcert_dst)
            print(f"  Copiado: mkcert.exe -> {mkcert_dst}")
        else:
            print(f"  ADVERTENCIA: mkcert.exe no encontrado en {mkcert_src}")
            print("  Descarga desde: https://github.com/FiloSottile/mkcert/releases/latest")
            print("  y coloca el exe como 'mkcert.exe' en la raiz del proyecto.")

        total_size = sum(f.stat().st_size for f in exe_folder.rglob('*') if f.is_file())
        size_mb = total_size / (1024 * 1024)

        print(f"\nAplicacion generada:")
        print(f"  Ubicacion: {exe_folder}")
        print(f"  Ejecutable: {exe_path}")
        print(f"  Tamano total: {size_mb:.2f} MB")

        print("\nPROXIMOS PASOS:")
        print("=" * 60)
        print("1. Probar el ejecutable:")
        print(f'   cd "{exe_folder}"')
        print(f"   .\\{OUTPUT_NAME}.exe")
        print()
        print("2. Verificar que el API funciona:")
        print("   http://localhost:58181/docs")
        print()
        print("3. Compilar instalador con Inno Setup:")
        print('   & "C:\\Program Files (x86)\\Inno Setup 6\\ISCC.exe" installer.iss')
        print("=" * 60)

    else:
        print(f"\nADVERTENCIA: No se encontro el ejecutable en {exe_path}")

except subprocess.CalledProcessError as e:
    print("\n" + "=" * 60)
    print("ERROR EN LA COMPILACION")
    print("=" * 60)
    print(f"\nCodigo de error: {e.returncode}")
    print("\nRevisa los errores arriba y asegurate de que:")
    print("1. Todas las dependencias esten instaladas")
    print("2. No haya errores de sintaxis en el codigo")
    print("3. PyInstaller este correctamente instalado")
    sys.exit(1)
except KeyboardInterrupt:
    print("\n\nCompilacion cancelada por el usuario")
    sys.exit(1)
