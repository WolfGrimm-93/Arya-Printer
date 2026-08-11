; installer/installer.iss — Inno Setup script for Arya Printer Service (Go).
;
; Reemplaza arya_escpos/installer.iss (Python/PyInstaller). Estructura de
; selector de directorio, grupo de Panel de Control, idioma español e
; instalación silenciosa se mantienen iguales a proposito (mismo flujo para
; el operador que ya conoce el instalador viejo). Lo que cambia, todo por
; hallazgos de la auditoria de seguridad del servicio Python:
;
;   1. [Run] instala un SERVICIO DE WINDOWS real via
;      "aryaprinter.exe --install-service" (installer/svcinstall, cuenta
;      virtual NT SERVICE\AryaESCPOSService) en vez de una Tarea Programada
;      corriendo como SYSTEM.
;   2. Después de instalar el servicio (orden obligatorio: la cuenta virtual
;      NT SERVICE\AryaESCPOSService solo existe una vez que el servicio está
;      registrado) se corre set_secure_acl.ps1, que dejar logs/configs/ssl/
;      secrets con permisos SOLO para esa cuenta + Administradores.
;   3. Se eliminó "Permissions: everyone-full/everyone-modify" de [Dirs] —
;      cualquier usuario local podia leer/escribir/borrar esas carpetas
;      (incluida la API key en secrets/) en el instalador viejo.
;   4. La excepción de firewall es una Task OPCIONAL, DESTILDADA por
;      defecto (antes: tildada por defecto) — setup_firewall.ps1 además
;      solo crea la regla si server.host no es loopback, ver ese script.
;
#define MyAppName "Arya Printer Service"
#define MyAppVersion "2.0.0"
#define MyAppPublisher "Arya Development"
#define MyAppURL "https://github.com/tu-usuario/arya-printer"
#define MyAppExeName "aryaprinter.exe"

[Setup]
AppId={{2C6E7B1A-9F3D-4A6E-8C2B-5E1D4F0A7B33}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}

DefaultDirName={autopf}\AryaPrinter
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes

; Requiere Administrador: instalar un servicio de Windows, correr icacls y
; crear reglas de firewall exigen elevación.
PrivilegesRequired=admin
PrivilegesRequiredOverridesAllowed=dialog

OutputDir=installer_output
OutputBaseFilename=AryaPrinterService_Setup_v{#MyAppVersion}
Compression=lzma2/max
SolidCompression=yes

WizardStyle=modern
SetupIconFile=compiler:SetupClassicIcon.ico
UninstallDisplayIcon={app}\{#MyAppExeName}

ArchitecturesInstallIn64BitMode=x64compatible

ShowLanguageDialog=no

[Languages]
Name: "spanish"; MessagesFile: "compiler:Languages\Spanish.isl"

[Tasks]
; Ninguna de las dos tildada por defecto: el operador las habilita a
; conciencia. El arranque automático del servicio ya no es opcional —
; installer/svcinstall.Install siempre registra el servicio como
; StartAutomatic, no hace falta una Task separada para eso.
Name: "firewall"; Description: "Agregar excepcion en el Firewall de Windows (solo si el servicio esta configurado para escuchar en una IP que no sea localhost — ver configs\settings.yaml: server.host)"; GroupDescription: "Configuracion de red:"
Name: "ssl"; Description: "Habilitar HTTPS en localhost (elimina advertencias del navegador Chrome/Edge/Firefox)"; GroupDescription: "Seguridad:"

[Files]
; Ejecutable principal. La ruta de origen asume un build previo con
; "go build -o dist\aryaprinter.exe .\cmd\aryaprinter" — ajustar si el
; pipeline de build usa otra carpeta de salida.
Source: "..\dist\aryaprinter.exe"; DestDir: "{app}"; Flags: ignoreversion

; Configuracion por defecto (el servicio la sobreescribe solo si el archivo
; no existe todavia — ver internal/config.Load).
Source: "..\configs\settings.yaml"; DestDir: "{app}\configs"; Flags: ignoreversion onlyifdoesntexist

; Driver USB para impresoras termicas sin cola de Windows.
Source: "..\libs\libusb-1.0.dll"; DestDir: "{app}\libs"; Flags: ignoreversion

; SumatraPDF portable (GPLv3, ver libs\THIRD-PARTY-NOTICES.md) empaquetado
; para que /print/document funcione sin que el operador instale nada aparte
; -- fallback de impresion de PDF cuando el renderizado nativo (PDFium) no
; esta disponible. internal/document/pdf_print.go lo busca en {app}\libs\
; antes que en Program Files.
Source: "..\libs\SumatraPDF.exe"; DestDir: "{app}\libs"; Flags: ignoreversion
Source: "..\libs\THIRD-PARTY-NOTICES.md"; DestDir: "{app}\libs"; Flags: ignoreversion

; mkcert para HTTPS local confiable (Chrome/Firefox/Edge sin advertencias).
Source: "..\mkcert.exe"; DestDir: "{app}"; Flags: ignoreversion

; Scripts de PowerShell propios (firewall condicional + ACL restrictiva).
Source: "setup_firewall.ps1"; DestDir: "{app}"; Flags: ignoreversion
Source: "remove_firewall.ps1"; DestDir: "{app}"; Flags: ignoreversion
Source: "set_secure_acl.ps1"; DestDir: "{app}"; Flags: ignoreversion

[Dirs]
; Sin "Permissions: everyone-*". Las carpetas se crean con la ACL heredada
; de {app} (solo Administradores/SYSTEM por defecto en Program Files) y
; luego set_secure_acl.ps1 las deja restringidas a la cuenta de servicio +
; Administradores unicamente — ver [Run] mas abajo para el orden exacto.
Name: "{app}\logs"
Name: "{app}\configs"
Name: "{app}\ssl"
Name: "{app}\secrets"

[Registry]
Root: HKLM; Subkey: "Software\{#MyAppPublisher}\{#MyAppName}"; Flags: uninsdeletekeyifempty
Root: HKLM; Subkey: "Software\{#MyAppPublisher}\{#MyAppName}"; ValueType: string; ValueName: "InstallPath"; ValueData: "{app}"; Flags: uninsdeletevalue
Root: HKLM; Subkey: "Software\{#MyAppPublisher}\{#MyAppName}"; ValueType: string; ValueName: "Version"; ValueData: "{#MyAppVersion}"; Flags: uninsdeletevalue

[Run]
; 1) Instalar y arrancar el servicio de Windows (cuenta virtual
;    NT SERVICE\AryaESCPOSService, ver installer/svcinstall/install_windows.go).
;    El nombre interno del servicio (AryaESCPOSService) esta fijado por
;    cmd/aryaprinter/service_windows.go (Agente A) -- NO es "AryaPrinter";
;    ese es solo el nombre del producto/carpeta de instalacion.
;    DEBE correr ANTES que set_secure_acl.ps1: esa cuenta virtual solo es
;    resoluble por LSA una vez que el servicio esta registrado en el SCM
;    (verificado a mano con icacls contra un servicio inexistente: falla
;    con "No mapping between account names and security IDs was done.").
Filename: "{app}\{#MyAppExeName}"; Parameters: "--install-service"; Flags: runhidden waituntilterminated; StatusMsg: "Instalando el servicio de Windows..."

; 2) Restringir permisos de logs/configs/ssl/secrets a la cuenta de
;    servicio + Administradores unicamente (corrige everyone-full/modify
;    del instalador Python).
Filename: "{sys}\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\set_secure_acl.ps1"" -InstallPath ""{app}"""; Flags: runhidden waituntilterminated; StatusMsg: "Restringiendo permisos de carpetas sensibles..."

; 3) Firewall: opcional, destildado por defecto. El script en si mismo
;    vuelve a chequear server.host antes de crear nada (defensa en
;    profundidad: aunque el operador tilde la Task con un host loopback en
;    la config, no se crea una regla que no hace falta).
Filename: "{sys}\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\setup_firewall.ps1"""; Tasks: firewall; Flags: runhidden waituntilterminated; StatusMsg: "Configurando Firewall de Windows..."

; 4) HTTPS local: opcional, destildado por defecto.
Filename: "{app}\{#MyAppExeName}"; Parameters: "--setup-ssl"; Tasks: ssl; Flags: runhidden waituntilterminated; StatusMsg: "Configurando HTTPS para localhost..."

; 5) Mensaje final: como obtener la API key que Arya Market necesita
;    configurar (ver README.md, seccion "Configurar la API key").
; Nota de escritura: dentro de un valor "..." de Inno Setup no existe el
; backslash como caracter de escape (solo sirve para separar carpetas en
; rutas) — la unica forma de meter una comilla doble literal es duplicarla
; (""). Por eso este comando evita anidar comillas dobles: usa solo
; comillas simples para los strings de PowerShell, con {app} y
; {#MyAppExeName} ya expandidos por Inno antes de que PowerShell vea el
; texto (no son una invocacion real, solo se muestran como instruccion).
Filename: "{sys}\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-ExecutionPolicy Bypass -Command ""Write-Host ''; Write-Host 'Instalacion completada' -ForegroundColor Green; Write-Host ''; Write-Host 'El servicio ya esta corriendo (NT SERVICE\AryaESCPOSService, no SYSTEM).'; Write-Host 'Para obtener la API key que hay que cargar en Arya Market, ejecuta:'; Write-Host '  {app}\{#MyAppExeName} --show-api-key' -ForegroundColor Cyan; Write-Host ''; pause"""; Flags: postinstall skipifsilent nowait; Description: "Ver como configurar la API key en Arya Market"

[UninstallRun]
; Desinstalar el servicio primero (detiene el proceso el mismo).
Filename: "{app}\{#MyAppExeName}"; Parameters: "--uninstall-service"; Flags: runhidden waituntilterminated; RunOnceId: "UninstallService"

; Por si el servicio no llegó a arrancar nunca y quedó el proceso suelto de
; una ejecución manual en desarrollo.
Filename: "{sys}\taskkill.exe"; Parameters: "/F /IM {#MyAppExeName}"; Flags: runhidden; RunOnceId: "StopProcess"

; Eliminar la regla de firewall si existe (idempotente si nunca se creo).
Filename: "{sys}\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\remove_firewall.ps1"""; Flags: runhidden waituntilterminated; RunOnceId: "DeleteFirewallRule"

; Eliminar CA de mkcert y los certificados, si se configuraron.
Filename: "{app}\{#MyAppExeName}"; Parameters: "--remove-ssl"; Flags: runhidden waituntilterminated; RunOnceId: "RemoveSSL"

[UninstallDelete]
Type: filesandordirs; Name: "{app}\logs"
Type: filesandordirs; Name: "{app}\ssl"
; secrets\ (la API key) y configs\ (settings.yaml con eventuales cambios
; del operador) se conservan a proposito en la desinstalacion, igual que en
; el instalador Python conservaba config/ — evita que una desinstalacion
; accidental invalide la API key que Arya Market ya tiene guardada, y que
; una reinstalacion sobre el mismo directorio pierda configuracion custom.

[Code]
// Verifica si el servicio de Windows AryaESCPOSService ya existe, usando
// "sc.exe query" (disponible en cualquier Windows, no requiere el modulo
// de PowerShell ServerManager). ResultCode 0 = el servicio existe.
function IsServiceInstalled(ServiceName: String): Boolean;
var
  ResultCode: Integer;
begin
  Exec(ExpandConstant('{sys}\sc.exe'), 'query "' + ServiceName + '"', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Result := (ResultCode = 0);
end;

// Chequeo de dependencias operativas opcionales: solo LibreOffice queda
// afuera del instalador (conversion DOCX/XLSX -> PDF), es un proyecto de
// terceros con su propio instalador y demasiado grande para empaquetar.
// SumatraPDF.exe SI viene empaquetado (ver seccion [Files], libs\SumatraPDF.exe)
// asi que ya no se chequea aca -- /print/document siempre tiene al menos un
// metodo de impresion de PDF disponible sin instalar nada aparte. Si falta
// LibreOffice, el servicio arranca igual -- solo falla la conversion de
// DOCX/XLSX especificamente, con 503, hasta que se instale.
function FileExistsAtAny(Paths: TArrayOfString): Boolean;
var
  I: Integer;
begin
  Result := False;
  for I := 0 to GetArrayLength(Paths) - 1 do
  begin
    if FileExists(Paths[I]) then
    begin
      Result := True;
      Exit;
    end;
  end;
end;

function CheckOptionalDependencies(): String;
var
  LibreOfficePaths: TArrayOfString;
  HasLibreOffice: Boolean;
  Missing: String;
begin
  LibreOfficePaths := ['C:\Program Files\LibreOffice\program\soffice.exe',
                        'C:\Program Files (x86)\LibreOffice\program\soffice.exe'];

  HasLibreOffice := FileExistsAtAny(LibreOfficePaths);

  Missing := '';
  if not HasLibreOffice then
    Missing := Missing + '  - LibreOffice (conversion de DOCX/XLSX a PDF)' + #13#10;

  Result := Missing;
end;

function InitializeSetup(): Boolean;
var
  ResultCode: Integer;
  MissingDeps: String;
begin
  Result := True;

  if IsServiceInstalled('AryaESCPOSService') then
  begin
    if MsgBox('El servicio Arya Printer ya esta instalado.' + #13#10 +
              'Deseas detenerlo y continuar con la instalacion (actualizacion)?',
              mbConfirmation, MB_YESNO) = IDYES then
    begin
      Exec(ExpandConstant('{sys}\sc.exe'), 'stop AryaESCPOSService', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
      Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /IM aryaprinter.exe', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
      Sleep(2000);
      // No se borra el servicio aca: --install-service en [Run] ya
      // reinstala limpio sobre cualquier registro previo (ver
      // installer/svcinstall.Install, es idempotente).
    end
    else
    begin
      Result := False;
    end;
  end;

  if Result then
  begin
    MissingDeps := CheckOptionalDependencies();
    if MissingDeps <> '' then
    begin
      MsgBox('Advertencia: no se encontraron en las rutas habituales los siguientes' + #13#10 +
             'componentes que POST /api/v1/print/document necesita en tiempo de ejecucion:' + #13#10 + #13#10 +
             MissingDeps + #13#10 +
             'El servicio se instala igual — esta funcionalidad especifica no va a' + #13#10 +
             'andar hasta que se instalen por separado. El resto del servicio' + #13#10 +
             '(tickets ESC/POS, ESC/P, reportes) no depende de ellos.',
             mbInformation, MB_OK);
    end;
  end;
end;

[Messages]
WelcomeLabel2=Esto instalara [name/ver] en su computadora.%n%nEste asistente configurara automaticamente el servicio de Windows (cuenta de bajo privilegio, no SYSTEM) y lo iniciara en segundo plano.
