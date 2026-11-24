#define AppName "RAG Studio"
#ifndef AppVersion
  #define AppVersion "0.1.1"
#endif

[Setup]
AppName={#AppName}
AppVersion={#AppVersion}
AppVerName={#AppName} {#AppVersion}
AppPublisher=RP-Guruh
AppPublisherURL=https://github.com/RP-Guruh/rag-studio
AppSupportURL=https://github.com/RP-Guruh/rag-studio/issues
DefaultDirName={autopf}\RAG Studio
DefaultGroupName={#AppName}
OutputBaseFilename=rag-studio-{#AppVersion}-windows-amd64-setup
OutputDir=..\..\release
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
ArchitecturesInstallIn64BitMode=x64
CloseApplications=yes

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Create a desktop icon"; GroupDescription: "Additional icons:"

[Files]
Source: "dist\rag-studio.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "README.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "deploy\local\docker-compose.yml"; DestDir: "{app}\deploy\local"; Flags: ignoreversion

[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\rag-studio.exe"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\rag-studio.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\rag-studio.exe"; Description: "Launch RAG Studio"; Flags: nowait postinstall skipifsilent
