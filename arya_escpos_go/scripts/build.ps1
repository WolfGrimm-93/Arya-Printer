# scripts/build.ps1
#
# Gate de verificacion antes de cualquier commit. Corre build + vet + test
# sobre todo el modulo y falla (exit 1) si alguno de los tres no sale limpio.
#
# Uso:
#   .\scripts\build.ps1
#
# Requiere Go 1.26+ instalado. Si "go" no esta en el PATH de esta sesion de
# PowerShell (comun en instalaciones frescas de Windows donde el PATH de
# Machine/User se actualizo pero la sesion actual no lo heredo), este script
# lo refresca automaticamente antes de invocar cualquier comando go.

$ErrorActionPreference = "Stop"

# Refrescar PATH desde el registro (Machine + User) para tomar el Go recien
# instalado sin depender de que la sesion de PowerShell se haya reabierto.
$env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")

function Test-GoAvailable {
    $null = Get-Command go -ErrorAction SilentlyContinue
    return $?
}

if (-not (Test-GoAvailable)) {
    Write-Host "ERROR: no se encontro 'go' en el PATH (ni en Machine ni en User)." -ForegroundColor Red
    Write-Host "Instala Go 1.26+ desde https://go.dev/dl/ o revisa la instalacion existente." -ForegroundColor Red
    exit 1
}

$goVersion = (go version)
Write-Host "Usando: $goVersion" -ForegroundColor Cyan

$steps = @(
    @{ Name = "go build ./..."; Args = @("build", "./...") },
    @{ Name = "go vet ./...";   Args = @("vet", "./...") },
    @{ Name = "go test ./...";  Args = @("test", "./...") }
)

$failed = $false

foreach ($step in $steps) {
    Write-Host ""
    Write-Host "==> $($step.Name)" -ForegroundColor Cyan
    & go @($step.Args)
    if ($LASTEXITCODE -ne 0) {
        Write-Host "FALLO: $($step.Name) (exit code $LASTEXITCODE)" -ForegroundColor Red
        $failed = $true
    } else {
        Write-Host "OK: $($step.Name)" -ForegroundColor Green
    }
}

Write-Host ""
if ($failed) {
    Write-Host "Verificacion FALLIDA. No commitear hasta corregir lo anterior." -ForegroundColor Red
    exit 1
}

Write-Host "Verificacion OK: build, vet y test limpios." -ForegroundColor Green
exit 0
