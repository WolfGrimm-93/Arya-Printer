# ========================================
# Script para instalar Arya ESCPOS como Tarea Programada de Windows
# Ejecutar como Administrador
# ========================================

Write-Host "Instalando Arya ESCPOS como Tarea Programada..." -ForegroundColor Cyan

# CONFIGURACIÓN - Ajusta estas rutas según tu instalación
$ProjectPath = Split-Path -Parent $MyInvocation.MyCommand.Path  # Ruta del script
$PythonExe = "$ProjectPath\venv\Scripts\python.exe"
$ScriptPath = "src\main.py"
$TaskName = "AryaESCPOS"

# Verificar que Python existe
if (-not (Test-Path $PythonExe)) {
    Write-Host "ERROR: No se encuentra Python en: $PythonExe" -ForegroundColor Red
    Write-Host "Verifica la ruta del virtual environment" -ForegroundColor Yellow
    exit 1
}

# Verificar que el script existe
if (-not (Test-Path "$ProjectPath\$ScriptPath")) {
    Write-Host "ERROR: No se encuentra el script en: $ProjectPath\$ScriptPath" -ForegroundColor Red
    exit 1
}

# Eliminar tarea existente si ya existe
$existingTask = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
if ($existingTask) {
    Write-Host "Eliminando tarea existente..." -ForegroundColor Yellow
    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
}

# Crear acción de la tarea
Write-Host "Configurando acción..." -ForegroundColor Green
$action = New-ScheduledTaskAction `
    -Execute $PythonExe `
    -Argument $ScriptPath `
    -WorkingDirectory $ProjectPath

# Crear trigger (inicio del sistema)
Write-Host "Configurando trigger de inicio..." -ForegroundColor Green
$trigger = New-ScheduledTaskTrigger `
    -AtStartup `
    -RandomDelay (New-TimeSpan -Seconds 30)

# Configuración de la tarea
Write-Host "Configurando opciones de la tarea..." -ForegroundColor Green
$settings = New-ScheduledTaskSettingsSet `
    -AllowStartIfOnBatteries `
    -DontStopIfGoingOnBatteries `
    -StartWhenAvailable `
    -RestartCount 999 `
    -RestartInterval (New-TimeSpan -Minutes 5) `
    -ExecutionTimeLimit (New-TimeSpan -Hours 0) `
    -Priority 4

# Principal (ejecutar como SYSTEM con privilegios máximos)
Write-Host "Configurando permisos (SYSTEM + Administrador)..." -ForegroundColor Green
$principal = New-ScheduledTaskPrincipal `
    -UserId "SYSTEM" `
    -LogonType ServiceAccount `
    -RunLevel Highest

# Registrar tarea
Write-Host "Registrando tarea programada..." -ForegroundColor Green
Register-ScheduledTask `
    -TaskName $TaskName `
    -Action $action `
    -Trigger $trigger `
    -Settings $settings `
    -Principal $principal `
    -Description "Servicio de impresión ESC/POS y Windows para Arya. Puerto 8181." `
    -ErrorAction Stop

Write-Host "`nTarea '$TaskName' registrada exitosamente!" -ForegroundColor Green

# Preguntar si iniciar ahora
$start = Read-Host "`n¿Deseas iniciar la tarea ahora? (S/N)"
if ($start -eq "S" -or $start -eq "s") {
    Write-Host "Iniciando tarea..." -ForegroundColor Cyan
    Start-ScheduledTask -TaskName $TaskName
    
    # Esperar un poco y verificar estado
    Start-Sleep -Seconds 3
    
    $taskInfo = Get-ScheduledTask -TaskName $TaskName
    $taskState = $taskInfo.State
    
    Write-Host "`nEstado de la tarea: $taskState" -ForegroundColor $(if ($taskState -eq "Running") { "Green" } else { "Yellow" })
    
    # Verificar que el API responde
    Write-Host "`nVerificando que el API responde..." -ForegroundColor Cyan
    Start-Sleep -Seconds 2
    
    try {
        $response = Invoke-RestMethod -Uri "http://localhost:8181/api/v1/devices" -Method Get -TimeoutSec 5
        Write-Host "✓ API respondiendo correctamente!" -ForegroundColor Green
        Write-Host "✓ Swagger UI disponible en: http://localhost:8181/docs" -ForegroundColor Green
    } catch {
        Write-Host "⚠ El API aún no responde (puede tardar unos segundos en iniciar)" -ForegroundColor Yellow
        Write-Host "  Verifica manualmente en: http://localhost:8181/docs" -ForegroundColor Yellow
    }
} else {
    Write-Host "`nPuedes iniciar la tarea manualmente con:" -ForegroundColor Yellow
    Write-Host "  Start-ScheduledTask -TaskName '$TaskName'" -ForegroundColor White
}

Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "COMANDOS ÚTILES:" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Ver estado:      Get-ScheduledTask -TaskName '$TaskName'" -ForegroundColor White
Write-Host "Iniciar:         Start-ScheduledTask -TaskName '$TaskName'" -ForegroundColor White
Write-Host "Detener:         Stop-ScheduledTask -TaskName '$TaskName'" -ForegroundColor White
Write-Host "Deshabilitar:    Disable-ScheduledTask -TaskName '$TaskName'" -ForegroundColor White
Write-Host "Habilitar:       Enable-ScheduledTask -TaskName '$TaskName'" -ForegroundColor White
Write-Host "Eliminar:        Unregister-ScheduledTask -TaskName '$TaskName' -Confirm:`$false" -ForegroundColor White
Write-Host "Ver logs:        Get-Content '$ProjectPath\logs\arya_escpos.log' -Tail 50 -Wait" -ForegroundColor White
Write-Host "`nSwagger UI:      http://localhost:8181/docs" -ForegroundColor Green
Write-Host "========================================`n" -ForegroundColor Cyan
