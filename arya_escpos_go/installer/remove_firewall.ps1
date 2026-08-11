# installer/remove_firewall.ps1
#
# Elimina la regla de Firewall de Arya Printer Service, si existe. Se invoca
# desde installer.iss en [UninstallRun]. Idempotente: si la regla no existe
# (porque nunca se creo, ver setup_firewall.ps1 y su caso "host loopback"),
# no falla.

param(
    [string]$RuleName = "Arya Printer Service"
)

function Write-Log {
    param([string]$Message)
    Write-Host "[remove_firewall] $Message"
}

$rule = Get-NetFirewallRule -DisplayName $RuleName -ErrorAction SilentlyContinue

if (-not $rule) {
    Write-Log "No existe ninguna regla '$RuleName'. Nada que eliminar."
    exit 0
}

try {
    Remove-NetFirewallRule -DisplayName $RuleName -Confirm:$false -ErrorAction Stop
    Write-Log "Regla '$RuleName' eliminada."
    exit 0
} catch {
    Write-Log "ERROR al eliminar la regla: $_"
    exit 1
}
