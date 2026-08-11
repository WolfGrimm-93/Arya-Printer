# installer/setup_firewall.ps1
#
# Configura (o no) la excepcion de Firewall de Windows para Arya Printer
# Service. Reescribe arya_escpos/setup_firewall.ps1 corrigiendo el hallazgo
# de auditoria: el Python original creaba la regla SIEMPRE, sin restriccion
# de perfil ni de alcance, habilitada por defecto.
#
# Reglas de esta version:
#   1. CONDICIONAL a la config: si server.host en configs\settings.yaml es
#      loopback (127.0.0.1, localhost, ::1) el servicio solo es alcanzable
#      desde la propia PC y NO tiene sentido una regla de entrada -> no se
#      crea (y si existe una de una instalacion previa con otro host, se
#      elimina). Esto refleja el default real: server.host = 127.0.0.1.
#   2. Si server.host SI es una IP no-loopback (el operador decidio exponer
#      el servicio a la red), la regla se crea pero acotada:
#        -Profile Private,Domain   (NUNCA Public)
#      -RemoteAddress no se restringe a LocalSubnet por defecto: si el
#      caso de uso realmente requiere limitar el origen a la subred local,
#      pasa -RestrictToLocalSubnet y este script agrega -RemoteAddress
#      LocalSubnet a la regla. No es el default porque cambia con la red del
#      cliente (VLANs, VPN, etc.) y el instalador no puede asumirlo con
#      certeza; documentarlo aca en vez de intentar adivinarlo.
#   3. Se mantiene la logica de "no duplicar reglas existentes" del script
#      Python: si ya existe una regla correctamente configurada, no se toca.
#
# Uso tipico (invocado desde installer.iss, Task "firewall" opcional, NO
# marcada por defecto):
#   powershell -ExecutionPolicy Bypass -File setup_firewall.ps1
#
# Uso manual con restriccion de subred:
#   .\setup_firewall.ps1 -RestrictToLocalSubnet

param(
    [string]$RuleName = "Arya Printer Service",
    [int]$Port = 58181,
    [string]$ConfigPath = (Join-Path $PSScriptRoot "..\configs\settings.yaml"),
    [switch]$RestrictToLocalSubnet
)

function Write-Log {
    param([string]$Message)
    Write-Host "[setup_firewall] $Message"
}

# --- Leer server.host de configs\settings.yaml (parser minimo, no requiere
#     un modulo YAML: solo nos interesa una clave escalar dentro del bloque
#     "server:") ---
function Get-ConfiguredHost {
    param([string]$Path)

    if (-not (Test-Path $Path)) {
        Write-Log "No se encontro $Path, asumiendo host por defecto 127.0.0.1"
        return "127.0.0.1"
    }

    $inServerBlock = $false
    foreach ($line in Get-Content -Path $Path) {
        if ($line -match '^\s*server:\s*$') {
            $inServerBlock = $true
            continue
        }
        if ($inServerBlock) {
            # Salir del bloque server: si encontramos una linea de nivel raiz
            # (sin indentacion) que no sea un comentario ni una linea vacia.
            if ($line -match '^\S' -and $line -notmatch '^\s*#') {
                break
            }
            if ($line -match '^\s+host:\s*"?([^"\s#]+)"?') {
                return $matches[1]
            }
        }
    }

    Write-Log "No se encontro server.host en $Path, asumiendo 127.0.0.1"
    return "127.0.0.1"
}

function Test-Loopback {
    param([string]$HostValue)
    return $HostValue -in @("127.0.0.1", "localhost", "::1")
}

$configuredHost = Get-ConfiguredHost -Path $ConfigPath
Write-Log "server.host configurado: $configuredHost"

$existingRule = Get-NetFirewallRule -DisplayName $RuleName -ErrorAction SilentlyContinue

if (Test-Loopback -HostValue $configuredHost) {
    Write-Log "El servicio esta configurado en loopback ($configuredHost): no se necesita ninguna regla de entrada."
    if ($existingRule) {
        Write-Log "Eliminando regla existente de una configuracion previa no-loopback..."
        Remove-NetFirewallRule -DisplayName $RuleName -Confirm:$false -ErrorAction SilentlyContinue
    }
    exit 0
}

Write-Log "server.host no es loopback: el operador expuso el servicio a la red. Configurando regla acotada..."

$desiredProfiles = "Private,Domain"
$rule = Get-NetFirewallRule -DisplayName $RuleName -ErrorAction SilentlyContinue
if ($rule) {
    $portFilter = $rule | Get-NetFirewallPortFilter
    $sameConfig = (
        $portFilter.LocalPort -eq $Port -and
        $rule.Action -eq "Allow" -and
        $rule.Enabled -eq $true -and
        ($rule.Profile.ToString() -replace '\s', '') -eq ($desiredProfiles -replace '\s', '')
    )
    if ($sameConfig) {
        Write-Log "Regla ya existe y esta correctamente configurada. Sin cambios."
        exit 0
    }
    Write-Log "Regla existe pero difiere de la configuracion esperada. Eliminando para recrearla..."
    Remove-NetFirewallRule -DisplayName $RuleName -Confirm:$false -ErrorAction SilentlyContinue
}

Write-Log "Creando regla de firewall: puerto $Port, perfiles $desiredProfiles..."

try {
    $params = @{
        DisplayName = $RuleName
        Direction   = "Inbound"
        Action      = "Allow"
        Protocol    = "TCP"
        LocalPort   = $Port
        Profile     = @("Private", "Domain")
        Enabled     = "True"
        ErrorAction = "Stop"
    }
    if ($RestrictToLocalSubnet) {
        Write-Log "Restringiendo origen a la subred local (-RemoteAddress LocalSubnet)."
        $params["RemoteAddress"] = "LocalSubnet"
    }

    New-NetFirewallRule @params | Out-Null
    Write-Log "Regla creada exitosamente."
    exit 0
} catch {
    Write-Log "ERROR al crear regla: $_"
    exit 1
}
