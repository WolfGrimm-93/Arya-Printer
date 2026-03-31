# Script para limpiar reglas duplicadas del Firewall de Arya ESCPOS
# Ejecutar como Administrador

Write-Host ""
Write-Host "=== Limpieza de reglas duplicadas del Firewall ===" -ForegroundColor Cyan
Write-Host ""

# Verificar si se ejecuta como admin
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")
if (-not $isAdmin) {
    Write-Host "ERROR: Este script debe ejecutarse como Administrador" -ForegroundColor Red
    Write-Host ""
    Write-Host "Opciones:"
    Write-Host "1. Click derecho en PowerShell -> Ejecutar como Administrador"
    Write-Host "2. Luego ejecuta: powershell -ExecutionPolicy Bypass -File cleanup_firewall.ps1"
    Write-Host ""
    exit 1
}

Write-Host "Analizando reglas del firewall para 'Arya ESCPOS Service'..."
$rules = Get-NetFirewallRule -DisplayName "Arya ESCPOS Service" -ErrorAction SilentlyContinue

if ($rules.Count -eq 0) {
    Write-Host "No se encontraron reglas. El firewall está limpio." -ForegroundColor Green
    exit 0
}

Write-Host "Se encontraron $($rules.Count) reglas. Eliminando todas..." -ForegroundColor Yellow
Write-Host ""

try {
    Get-NetFirewallRule -DisplayName "Arya ESCPOS Service" | Remove-NetFirewallRule -Confirm:$false -ErrorAction Stop
    Write-Host "Reglas eliminadas exitosamente." -ForegroundColor Green

    Write-Host ""
    Write-Host "Agregando una sola regla correcta..." -ForegroundColor Cyan

    # Crear regla única y limpia
    New-NetFirewallRule `
        -DisplayName "Arya ESCPOS Service" `
        -Direction Inbound `
        -Action Allow `
        -Protocol TCP `
        -LocalPort 58181 `
        -Enabled True `
        -ErrorAction Stop | Out-Null

    Write-Host "Nueva regla creada exitosamente." -ForegroundColor Green
    Write-Host ""
    Write-Host "Estado final:" -ForegroundColor Cyan
    Get-NetFirewallRule -DisplayName "Arya ESCPOS Service" | Format-Table -Property DisplayName, Direction, Action, Enabled

} catch {
    Write-Host "ERROR: $_" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "Limpieza completada." -ForegroundColor Green
