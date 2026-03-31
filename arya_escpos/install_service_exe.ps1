# ========================================
# Script para instalar Arya ESCPOS (EJECUTABLE) como Tarea Programada
# Ejecutar como Administrador
# ========================================

Write-Host "Instalando Arya ESCPOS (Ejecutable) como Tarea Programada..." -ForegroundColor Cyan

# CONFIGURACIÓN
$InstallPath = "C:\Program Files\AryaESCPOS"
$TaskName = "AryaESCPOS"
$SourcePath = Split-Path -Parent $MyInvocation.MyCommand.Path
$SourceExePath = Join-Path $SourcePath "AryaESCPOS.exe"
$ExePath = Join-Path $InstallPath "AryaESCPOS.exe"

Write-Host "`n📂 Carpeta de origen: $SourcePath" -ForegroundColor White
Write-Host "📂 Carpeta de instalación: $InstallPath" -ForegroundColor White

# Verificar que el ejecutable existe en la carpeta de origen
if (-not (Test-Path $SourceExePath)) {
    Write-Host "`n❌ ERROR: No se encuentra el ejecutable en: $SourceExePath" -ForegroundColor Red
    Write-Host "`nAsegúrate de que:" -ForegroundColor Yellow
    Write-Host "1. Este script está dentro de la carpeta AryaESCPOS (junto al .exe)" -ForegroundColor Yellow
    Write-Host "2. Ya compilaste el proyecto con build_executable.py" -ForegroundColor Yellow
    exit 1
}

Write-Host "✅ Ejecutable encontrado en carpeta de origen" -ForegroundColor Green

# Copiar archivos a Program Files
Write-Host "`n📦 Copiando archivos a $InstallPath..." -ForegroundColor Cyan
try {
    # Crear carpeta si no existe
    if (-not (Test-Path $InstallPath)) {
        New-Item -ItemType Directory -Path $InstallPath -Force | Out-Null
        Write-Host "   ✓ Carpeta creada" -ForegroundColor Green
    } else {
        Write-Host "   ℹ Carpeta ya existe, actualizando archivos..." -ForegroundColor Yellow
    }
    
    # Copiar todos los archivos (excepto el script de instalación)
    Get-ChildItem -Path $SourcePath | Where-Object { $_.Name -ne "install_service_exe.ps1" } | ForEach-Object {
        Copy-Item -Path $_.FullName -Destination $InstallPath -Recurse -Force
    }
    Write-Host "   ✓ Archivos copiados exitosamente" -ForegroundColor Green
    
} catch {
    Write-Host "   ❌ ERROR al copiar archivos: $_" -ForegroundColor Red
    exit 1
}

# Eliminar tarea existente si ya existe
$existingTask = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
if ($existingTask) {
    Write-Host "`n⚠️  Tarea existente encontrada. Eliminando..." -ForegroundColor Yellow
    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
    Write-Host "✅ Tarea anterior eliminada" -ForegroundColor Green
}

# Crear acción de la tarea
Write-Host "`n🔧 Configurando tarea programada..." -ForegroundColor Cyan
$action = New-ScheduledTaskAction `
    -Execute $ExePath `
    -WorkingDirectory $InstallPath

# Crear trigger (inicio del sistema con delay)
$trigger = New-ScheduledTaskTrigger `
    -AtStartup `
    -RandomDelay (New-TimeSpan -Seconds 30)

# Configuración de la tarea
$settings = New-ScheduledTaskSettingsSet `
    -AllowStartIfOnBatteries `
    -DontStopIfGoingOnBatteries `
    -StartWhenAvailable `
    -RestartCount 999 `
    -RestartInterval (New-TimeSpan -Minutes 5) `
    -ExecutionTimeLimit (New-TimeSpan -Hours 0) `
    -Priority 4

# Principal (ejecutar como SYSTEM con privilegios máximos)
$principal = New-ScheduledTaskPrincipal `
    -UserId "SYSTEM" `
    -LogonType ServiceAccount `
    -RunLevel Highest

# Registrar tarea
Write-Host "📝 Registrando tarea..." -ForegroundColor Cyan
try {
    Register-ScheduledTask `
        -TaskName $TaskName `
        -Action $action `
        -Trigger $trigger `
        -Settings $settings `
        -Principal $principal `
        -Description "Servicio de impresión ESC/POS y Windows para Arya. Puerto 8181." `
        -ErrorAction Stop
    
    Write-Host "`n✅ Tarea '$TaskName' registrada exitosamente!" -ForegroundColor Green
} catch {
    Write-Host "`n❌ ERROR al registrar la tarea:" -ForegroundColor Red
    Write-Host $_.Exception.Message -ForegroundColor Red
    exit 1
}

# Configurar firewall (opcional)
Write-Host "`n🔥 ¿Deseas abrir el puerto 8181 en el firewall? (S/N)" -ForegroundColor Yellow
$firewall = Read-Host "Respuesta"
if ($firewall -eq "S" -or $firewall -eq "s") {
    Write-Host "Configurando regla de firewall..." -ForegroundColor Cyan
    try {
        New-NetFirewallRule -DisplayName "Arya ESCPOS API" `
            -Direction Inbound `
            -LocalPort 8181 `
            -Protocol TCP `
            -Action Allow `
            -ErrorAction Stop
        Write-Host "✅ Puerto 8181 abierto en firewall" -ForegroundColor Green
    } catch {
        Write-Host "⚠️  Advertencia: No se pudo configurar firewall (puede que ya exista la regla)" -ForegroundColor Yellow
    }
}

# Preguntar si iniciar ahora
Write-Host "`n🚀 ¿Deseas iniciar el servicio ahora? (S/N)" -ForegroundColor Yellow
$start = Read-Host "Respuesta"
if ($start -eq "S" -or $start -eq "s") {
    Write-Host "`nIniciando servicio..." -ForegroundColor Cyan
    Start-ScheduledTask -TaskName $TaskName
    
    # Esperar un poco y verificar estado
    Start-Sleep -Seconds 3
    
    $taskInfo = Get-ScheduledTask -TaskName $TaskName
    $taskState = $taskInfo.State
    
    Write-Host "Estado de la tarea: $taskState" -ForegroundColor $(if ($taskState -eq "Running") { "Green" } else { "Yellow" })
    
    # Verificar que el API responde
    Write-Host "`nVerificando que el API responde (esperando 5 segundos)..." -ForegroundColor Cyan
    Start-Sleep -Seconds 5
    
    try {
        $response = Invoke-RestMethod -Uri "http://localhost:8181/api/v1/devices" -Method Get -TimeoutSec 10
        Write-Host "`n✅ ¡API respondiendo correctamente!" -ForegroundColor Green
        Write-Host "✅ Swagger UI disponible en: http://localhost:8181/docs" -ForegroundColor Green
        
        # Preguntar si escanear impresoras
        Write-Host "`n🖨️  ¿Deseas escanear impresoras ahora? (S/N)" -ForegroundColor Yellow
        $scan = Read-Host "Respuesta"
        if ($scan -eq "S" -or $scan -eq "s") {
            Write-Host "`nEscaneando impresoras..." -ForegroundColor Cyan
            $scanResult = Invoke-RestMethod -Uri "http://localhost:8181/api/v1/devices/scan" -Method Get -TimeoutSec 30
            Write-Host "✅ Encontradas: $($scanResult.found) impresoras" -ForegroundColor Green
            
            if ($scanResult.found -gt 0) {
                Write-Host "`nImpresoras detectadas:" -ForegroundColor White
                foreach ($device in $scanResult.devices) {
                    Write-Host "  - $($device.name) ($($device.type))" -ForegroundColor Cyan
                }
            }
        }
        
    } catch {
        Write-Host "`n⚠️  El API aún no responde" -ForegroundColor Yellow
        Write-Host "Esto es normal si es la primera vez que se ejecuta." -ForegroundColor Yellow
        Write-Host "Verifica manualmente en unos segundos: http://localhost:8181/docs" -ForegroundColor Yellow
        Write-Host "`nSi no inicia, revisa los logs en:" -ForegroundColor Yellow
        Write-Host "$CurrentPath\logs\arya_escpos.log" -ForegroundColor White
    }
} else {
    Write-Host "`nPuedes iniciar el servicio manualmente con:" -ForegroundColor Yellow
    Write-Host "  Start-ScheduledTask -TaskName '$TaskName'" -ForegroundColor White
}

Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "📋 INSTALACIÓN COMPLETADA" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "`nCOMANDOS ÚTILES:" -ForegroundColor White
Write-Host "----------------------------------------" -ForegroundColor Cyan
Write-Host "Ver estado:      Get-ScheduledTask -TaskName '$TaskName'" -ForegroundColor White
Write-Host "Iniciar:         Start-ScheduledTask -TaskName '$TaskName'" -ForegroundColor White
Write-Host "Detener:         Stop-ScheduledTask -TaskName '$TaskName'" -ForegroundColor White
Write-Host "Reiniciar:       Restart-ScheduledTask -TaskName '$TaskName'" -ForegroundColor White
Write-Host "Deshabilitar:    Disable-ScheduledTask -TaskName '$TaskName'" -ForegroundColor White
Write-Host "Habilitar:       Enable-ScheduledTask -TaskName '$TaskName'" -ForegroundColor White
Write-Host "Eliminar:        Unregister-ScheduledTask -TaskName '$TaskName' -Confirm:`$false" -ForegroundColor White
Write-Host "`nVer logs:" -ForegroundColor White
Write-Host "  Get-Content '$CurrentPath\logs\arya_escpos.log' -Tail 50 -Wait" -ForegroundColor Gray
Write-Host "`nACCESOS:" -ForegroundColor White
Write-Host "  Swagger UI:    http://localhost:8181/docs" -ForegroundColor Cyan
Write-Host "  API Base:      http://localhost:8181/api/v1" -ForegroundColor Cyan
Write-Host "========================================`n" -ForegroundColor Cyan
