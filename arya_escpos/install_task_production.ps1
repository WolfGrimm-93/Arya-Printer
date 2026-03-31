# ========================================
# Script para instalar Arya ESCPOS como Tarea Programada de Windows (PRODUCCIÓN)
# Este script se ejecuta durante la instalación con Inno Setup
# ========================================

param(
    [string]$InstallPath = $PSScriptRoot
)

# Configuración
$ExePath = Join-Path $InstallPath "AryaESCPOS.exe"
$TaskName = "AryaESCPOS"
$TaskDescription = "Servicio de gestión de impresoras ESC/POS y Windows para Arya. Puerto 58181."

# Logging
$logFile = Join-Path $InstallPath "logs\install_task.log"
New-Item -ItemType Directory -Force -Path (Join-Path $InstallPath "logs") | Out-Null

function Write-Log {
    param($Message)
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    "$timestamp - $Message" | Out-File -FilePath $logFile -Append
}

Write-Log "=== Iniciando instalación de tarea programada ==="
Write-Log "Ruta de instalación: $InstallPath"
Write-Log "Ejecutable: $ExePath"

# Verificar que el ejecutable existe
if (-not (Test-Path $ExePath)) {
    Write-Log "ERROR: No se encuentra el ejecutable en: $ExePath"
    exit 1
}
Write-Log "Ejecutable encontrado correctamente"

# Eliminar tarea existente si ya existe
try {
    $existingTask = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    if ($existingTask) {
        Write-Log "Eliminando tarea existente..."
        Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction Stop
        Write-Log "Tarea existente eliminada"
    }
} catch {
    Write-Log "Error al eliminar tarea existente: $_"
}

# Detener proceso si está corriendo
try {
    $process = Get-Process -Name "AryaESCPOS" -ErrorAction SilentlyContinue
    if ($process) {
        Write-Log "Deteniendo proceso existente..."
        Stop-Process -Name "AryaESCPOS" -Force -ErrorAction Stop
        Start-Sleep -Seconds 2
        Write-Log "Proceso detenido"
    }
} catch {
    Write-Log "Error al detener proceso: $_"
}

# Crear acción de la tarea
Write-Log "Configurando acción de la tarea..."
$action = New-ScheduledTaskAction `
    -Execute $ExePath `
    -WorkingDirectory $InstallPath

# Crear trigger (inicio del sistema)
Write-Log "Configurando trigger de inicio automático..."
$trigger = New-ScheduledTaskTrigger `
    -AtStartup `
    -RandomDelay (New-TimeSpan -Seconds 10)

# Configuración de la tarea
Write-Log "Configurando opciones de reinicio y disponibilidad..."
$settings = New-ScheduledTaskSettingsSet `
    -AllowStartIfOnBatteries `
    -DontStopIfGoingOnBatteries `
    -StartWhenAvailable `
    -RestartCount 999 `
    -RestartInterval (New-TimeSpan -Minutes 1) `
    -ExecutionTimeLimit (New-TimeSpan -Hours 0) `
    -Priority 4 `
    -MultipleInstances IgnoreNew

# Principal (ejecutar como SYSTEM con privilegios máximos)
Write-Log "Configurando permisos (SYSTEM con RunLevel Highest)..."
$principal = New-ScheduledTaskPrincipal `
    -UserId "SYSTEM" `
    -LogonType ServiceAccount `
    -RunLevel Highest

# Registrar tarea
try {
    Write-Log "Registrando tarea programada en Task Scheduler..."
    Register-ScheduledTask `
        -TaskName $TaskName `
        -Action $action `
        -Trigger $trigger `
        -Settings $settings `
        -Principal $principal `
        -Description $TaskDescription `
        -ErrorAction Stop | Out-Null
    
    Write-Log "Tarea '$TaskName' registrada exitosamente"
    
    # Verificar que se creó
    $task = Get-ScheduledTask -TaskName $TaskName -ErrorAction Stop
    Write-Log "Verificación: Tarea encontrada en Task Scheduler - Estado: $($task.State)"
    
    exit 0
    
} catch {
    Write-Log "ERROR al registrar tarea: $_"
    Write-Log "Detalles: $($_.Exception.Message)"
    exit 1
}
