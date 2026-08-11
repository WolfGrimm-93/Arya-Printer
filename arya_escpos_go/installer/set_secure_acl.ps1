# installer/set_secure_acl.ps1
#
# Restringe los permisos de las carpetas sensibles de Arya Printer Service
# (logs, configs, ssl, secrets) a la cuenta de servicio + Administradores
# unicamente. Reemplaza el "Permissions: everyone-full" / "everyone-modify"
# que tenia arya_escpos/installer.iss (hallazgo de auditoria: cualquier
# usuario local podia leer/escribir/borrar logs, configuracion, certificados
# SSL y, mas grave, la API key en secrets/apikey.key).
#
# Se invoca desde installer.iss ([Run], despues de que existan las carpetas)
# y opcionalmente puede correrse a mano para re-aplicar permisos si alguien
# los toco manualmente.
#
# La cuenta de servicio (NT SERVICE\AryaESCPOSService) debe coincidir
# exactamente con ServiceName en installer/svcinstall/install_windows.go —
# ahi vive el nombre canonico (svcinstall.VirtualAccount); este script lo
# repite como string porque un .ps1 no puede importar un paquete Go. Ese
# nombre a su vez esta fijado por cmd/aryaprinter/service_windows.go
# (Agente A, const serviceName), no es arbitrario: cambia solo si ese
# archivo cambia.
#
# IMPORTANTE - orden de instalacion: la cuenta virtual "NT SERVICE\X" solo es
# resoluble por LSA (LookupAccountName) mientras exista un servicio de
# Windows llamado exactamente X registrado en el SCM. Verificado a mano:
# "icacls ... /grant NT SERVICE\NoSuchService:..." falla con "No mapping
# between account names and security IDs was done." si el servicio no esta
# instalado. Por eso este script SIEMPRE debe correr DESPUES de
# "aryaprinter.exe --install-service" (svcinstall.Install), nunca antes -
# installer.iss respeta ese orden en [Run].
#
# Uso:
#   powershell -ExecutionPolicy Bypass -File set_secure_acl.ps1 -InstallPath "C:\Program Files\AryaPrinter"

param(
    [string]$InstallPath = $PSScriptRoot,
    [string]$ServiceAccount = "NT SERVICE\AryaESCPOSService",
    [string]$ServiceName = "AryaESCPOSService"
)

$ErrorActionPreference = "Stop"

function Write-Log {
    param([string]$Message)
    Write-Host "[set_secure_acl] $Message"
}

# Chequeo temprano con mensaje claro en vez de dejar que icacls falle mas
# abajo con "No mapping between account names and security IDs was done."
# (ver nota de orden de instalacion arriba).
if (-not (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue)) {
    throw "El servicio de Windows '$ServiceName' no esta instalado. Ejecuta 'aryaprinter.exe --install-service' antes de aplicar estos permisos: la cuenta virtual $ServiceAccount no existe hasta que el servicio esta registrado."
}

# Administradores y SYSTEM via SID bien conocido (well-known SID), no por
# nombre: el nombre del grupo "Administrators"/"Administradores" cambia
# segun el idioma de Windows, el SID no.
$AdministratorsSid = "*S-1-5-32-544"
$SystemSid = "*S-1-5-18"

$targetDirs = @("logs", "configs", "ssl", "secrets")

foreach ($dirName in $targetDirs) {
    $path = Join-Path $InstallPath $dirName

    if (-not (Test-Path $path)) {
        Write-Log "Creando $path"
        New-Item -ItemType Directory -Path $path -Force | Out-Null
    }

    Write-Log "Aplicando ACL restrictiva a $path"

    # /inheritance:r  -> corta la herencia y elimina TODAS las entradas
    #                    heredadas (incluye "Users"/"Everyone" heredados de
    #                    {app} o de la unidad, que es exactamente lo que
    #                    corrige el hallazgo de auditoria).
    # /grant:r "X:F"  -> reemplaza (no acumula) los permisos explicitos de X.
    #   (OI)(CI)  -> se aplica a subcarpetas y archivos nuevos tambien.
    #
    # OJO: no redirigir el stderr de icacls con "2>&1" aca. Verificado a
    # mano: con $ErrorActionPreference = "Stop" (como en este script), un
    # "2>&1" sobre un exe nativo lo envuelve en un NativeCommandError que
    # aborta el script en esa misma linea (PowerShell 5.1), antes de que el
    # chequeo de $LASTEXITCODE de abajo llegue a ejecutarse - dejando un
    # error generico en vez del mensaje especifico que arma este script. Sin
    # redireccion, icacls imprime directo a consola y $LASTEXITCODE se puede
    # chequear normalmente.
    icacls $path /inheritance:r | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "icacls /inheritance:r fallo en $path (exit $LASTEXITCODE)"
    }

    icacls $path /grant:r "${SystemSid}:(OI)(CI)F" | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "icacls /grant SYSTEM fallo en $path (exit $LASTEXITCODE)"
    }

    icacls $path /grant:r "${AdministratorsSid}:(OI)(CI)F" | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "icacls /grant Administradores fallo en $path (exit $LASTEXITCODE)"
    }

    # La cuenta de servicio solo necesita Modify (leer/escribir/crear/borrar
    # sus propios archivos), no Full Control (no necesita cambiar permisos).
    icacls $path /grant:r "${ServiceAccount}:(OI)(CI)M" | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "icacls /grant $ServiceAccount fallo en $path (exit $LASTEXITCODE)"
    }

    Write-Log "OK: $path -> SYSTEM:F, Administradores:F, ${ServiceAccount}:M (sin Everyone/Users)"
}

Write-Log "Permisos aplicados correctamente."
exit 0
