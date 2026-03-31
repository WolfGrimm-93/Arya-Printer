# Script para configurar firewall de Arya ESCPOS (inteligente)
# Verifica si existe la regla antes de crear

param(
    [string]$RuleName = "Arya ESCPOS Service",
    [int]$Port = 58181
)

Write-Log "Verificando regla del firewall: $RuleName"

# Verificar si la regla existe
$rule = Get-NetFirewallRule -DisplayName $RuleName -ErrorAction SilentlyContinue

if ($rule) {
    # Verificar si está correctamente configurada
    $ruleDetail = Get-NetFirewallRule -DisplayName $RuleName | Get-NetFirewallPortFilter

    if ($ruleDetail.LocalPort -eq $Port -and $rule.Action -eq "Allow" -and $rule.Enabled -eq $true) {
        Write-Log "Regla ya existe y está correctamente configurada. Saltando..."
        exit 0
    } else {
        Write-Log "Regla existe pero está mal configurada. Eliminando para recrearla..."
        Remove-NetFirewallRule -DisplayName $RuleName -Confirm:$false -ErrorAction SilentlyContinue
    }
}

# Si llegamos aquí, creamos la regla
Write-Log "Creando nueva regla del firewall..."

try {
    New-NetFirewallRule `
        -DisplayName $RuleName `
        -Direction Inbound `
        -Action Allow `
        -Protocol TCP `
        -LocalPort $Port `
        -Enabled True `
        -ErrorAction Stop | Out-Null

    Write-Log "Regla creada exitosamente"
    exit 0
} catch {
    Write-Log "ERROR al crear regla: $_"
    exit 1
}
