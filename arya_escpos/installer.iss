; Script de Inno Setup para Arya ESCPOS Service
; Instalador profesional con servicio de Windows automático

#define MyAppName "Arya ESCPOS Service"
#define MyAppVersion "1.3.0"
#define MyAppPublisher "Arya Development"
#define MyAppURL "https://github.com/tu-usuario/arya-escpos"
#define MyAppExeName "AryaESCPOS.exe"
#define MyServiceName "AryaESCPOS"
#define MyServiceDisplayName "Arya ESCPOS Service"
#define MyServiceDescription "Servicio de gestión de impresoras ESC/POS"

[Setup]
; Información básica
AppId={{8F9A2B3C-4D5E-6F7A-8B9C-0D1E2F3A4B5C}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}

; Directorio de instalación
DefaultDirName={autopf}\AryaESCPOS
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes

; Permisos de administrador requeridos
PrivilegesRequired=admin
PrivilegesRequiredOverridesAllowed=dialog

; Salida
OutputDir=installer_output
OutputBaseFilename=AryaESCPOS_Setup_v{#MyAppVersion}
Compression=lzma2/max
SolidCompression=yes

; Aspecto visual
WizardStyle=modern
SetupIconFile=compiler:SetupClassicIcon.ico
UninstallDisplayIcon={app}\{#MyAppExeName}

; Arquitectura
ArchitecturesInstallIn64BitMode=x64compatible

; Licencia (opcional)
; LicenseFile=LICENSE

; Idioma
ShowLanguageDialog=no

[Languages]
Name: "spanish"; MessagesFile: "compiler:Languages\Spanish.isl"

[Tasks]
Name: "autostart"; Description: "Iniciar servicio automáticamente al arrancar Windows (recomendado)"; GroupDescription: "Opciones de servicio:"; Flags: checkedonce
Name: "firewall"; Description: "Agregar excepción en el Firewall de Windows (puerto 58181)"; GroupDescription: "Configuración de red:"; Flags: checkedonce
Name: "ssl"; Description: "Habilitar HTTPS en localhost (elimina advertencias del navegador Chrome)"; GroupDescription: "Seguridad:";
Name: "scanprinters"; Description: "Escanear impresoras disponibles después de la instalación"; GroupDescription: "Configuración inicial:";

[Files]
; Ejecutable principal y todos los archivos internos
Source: "dist\AryaESCPOS\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

; Archivos de configuración adicionales
Source: "config\*"; DestDir: "{app}\config"; Flags: ignoreversion recursesubdirs createallsubdirs

; Carpeta libs con libusb
Source: "libs\*"; DestDir: "{app}\libs"; Flags: ignoreversion recursesubdirs createallsubdirs

; Scripts de PowerShell
Source: "install_task_production.ps1"; DestDir: "{app}"; Flags: ignoreversion
Source: "setup_firewall.ps1"; DestDir: "{app}"; Flags: ignoreversion

[Dirs]
; Crear carpetas necesarias con permisos completos
Name: "{app}\logs"; Permissions: everyone-full
Name: "{app}\config"; Permissions: everyone-modify
Name: "{app}\ssl"; Permissions: everyone-modify

[Registry]
; Registrar la aplicación
Root: HKLM; Subkey: "Software\{#MyAppPublisher}\{#MyAppName}"; Flags: uninsdeletekeyifempty
Root: HKLM; Subkey: "Software\{#MyAppPublisher}\{#MyAppName}"; ValueType: string; ValueName: "InstallPath"; ValueData: "{app}"; Flags: uninsdeletevalue
Root: HKLM; Subkey: "Software\{#MyAppPublisher}\{#MyAppName}"; ValueType: string; ValueName: "Version"; ValueData: "{#MyAppVersion}"; Flags: uninsdeletevalue

[Run]
; Crear la tarea programada de Windows usando PowerShell (solo si autostart seleccionado)
Filename: "{sys}\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\install_task_production.ps1"" -InstallPath ""{app}"""; Tasks: autostart; Flags: runhidden waituntilterminated; StatusMsg: "Creando tarea programada de Windows..."

; Iniciar la tarea programada inmediatamente después de crearla
Filename: "{sys}\schtasks.exe"; Parameters: "/Run /TN ""AryaESCPOS"""; Tasks: autostart; Flags: runhidden waituntilterminated; StatusMsg: "Iniciando servicio..."

; Configurar excepción al firewall (si la tarea está seleccionada)
; Script verifica si existe y solo crea si no está
Filename: "{sys}\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\setup_firewall.ps1"""; Tasks: firewall; Flags: runhidden waituntilterminated; StatusMsg: "Configurando Firewall de Windows..."

; Generar certificado SSL e instalar CA en Windows (si la tarea está seleccionada)
Filename: "{app}\{#MyAppExeName}"; Parameters: "--setup-ssl"; Tasks: ssl; Flags: runhidden waituntilterminated; StatusMsg: "Configurando HTTPS para localhost..."

; Esperar 5 segundos para que el servicio esté completamente iniciado (solo si autostart)
Filename: "{sys}\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-ExecutionPolicy Bypass -Command ""Start-Sleep -Seconds 5"""; Tasks: autostart; Flags: runhidden waituntilterminated; StatusMsg: "Esperando inicio del servicio..."

; Escanear impresoras (requiere que el servicio esté corriendo)
Filename: "{sys}\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-ExecutionPolicy Bypass -Command ""try {{ Invoke-RestMethod -Uri 'http://localhost:58181/api/v1/devices/scan' -Method GET -TimeoutSec 10 }} catch {{ }}"""; Tasks: autostart scanprinters; Flags: runhidden waituntilterminated; StatusMsg: "Escaneando impresoras disponibles..."

; Mostrar mensaje final
Filename: "{sys}\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-ExecutionPolicy Bypass -Command ""Write-Host ''; Write-Host 'Instalacion completada exitosamente' -ForegroundColor Green; Write-Host ''; Write-Host 'Servicio iniciado en segundo plano'; Write-Host 'API Documentation: http://localhost:58181/docs' -ForegroundColor Cyan; Write-Host ''; Write-Host 'El servicio se iniciara automaticamente al encender la PC' -ForegroundColor Yellow; Write-Host ''; pause"""; Flags: postinstall skipifsilent nowait; Description: "Ver información del servicio"

[UninstallRun]
; Detener el proceso primero
Filename: "{sys}\taskkill.exe"; Parameters: "/F /IM AryaESCPOS.exe"; Flags: runhidden; RunOnceId: "StopProcess"

; Esperar 2 segundos para que el proceso se detenga completamente
Filename: "{sys}\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-Command ""Start-Sleep -Seconds 2"""; Flags: runhidden waituntilterminated; RunOnceId: "WaitForStop"

; Eliminar la tarea programada de Windows
Filename: "{sys}\schtasks.exe"; Parameters: "/Delete /TN ""AryaESCPOS"" /F"; Flags: runhidden; RunOnceId: "DeleteTask"

; Eliminar regla del firewall (si existe)
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall delete rule name=""Arya ESCPOS Service"""; Flags: runhidden; RunOnceId: "DeleteFirewallRule"

; Eliminar certificado SSL de Windows (si existe)
Filename: "{app}\{#MyAppExeName}"; Parameters: "--remove-ssl"; Flags: runhidden waituntilterminated; RunOnceId: "RemoveSSL"

[UninstallDelete]
; Eliminar logs y certificados SSL
Type: filesandordirs; Name: "{app}\logs"
Type: filesandordirs; Name: "{app}\ssl"

[Code]
// Función para verificar si la tarea programada existe
function IsTaskScheduled(TaskName: String): Boolean;
var
  ResultCode: Integer;
begin
  // Ejecutar schtasks query para verificar si existe la tarea
  Exec(ExpandConstant('{sys}\schtasks.exe'), '/Query /TN "' + TaskName + '"', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  // Si ResultCode = 0, la tarea existe
  Result := (ResultCode = 0);
end;

// Función que se ejecuta antes de la instalación
function InitializeSetup(): Boolean;
var
  ResultCode: Integer;
begin
  Result := True;
  
  // Verificar si la tarea programada ya existe
  if IsTaskScheduled('AryaESCPOS') then
  begin
    if MsgBox('El servicio Arya ESCPOS ya está instalado.' + #13#10 + 
              '¿Desea detenerlo y continuar con la instalación?', 
              mbConfirmation, MB_YESNO) = IDYES then
    begin
      // Eliminar la tarea programada existente
      Exec(ExpandConstant('{sys}\schtasks.exe'), '/Delete /TN "AryaESCPOS" /F', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
      // Detener cualquier proceso en ejecución
      Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /IM AryaESCPOS.exe', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
      Sleep(2000);
    end
    else
    begin
      Result := False;
    end;
  end;
end;

[Messages]
WelcomeLabel2=Esto instalará [name/ver] en su computadora.%n%nEste asistente configurará automáticamente el servicio de Windows y lo iniciará en segundo plano.
