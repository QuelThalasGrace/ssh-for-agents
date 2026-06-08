$BaseUrl = if ($env:SFA_BASE_URL) { $env:SFA_BASE_URL } else { "https://github.com/QuelThalasGrace/ssh-for-agents/releases/latest/download" }
$InstallDir = Join-Path $HOME ".ssh-for-agents\bin"
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

$arch = if ([System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture -eq "Arm64") { "arm64" } else { "amd64" }
$url = "$BaseUrl/sfa-windows-$arch.exe"
$out = Join-Path $InstallDir "sfa.exe"

Invoke-WebRequest -Uri $url -OutFile $out

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$InstallDir*") {
  [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
  Write-Host "[warn] PATH updated. Restart your terminal to use sfa."
}

Write-Host "[ok] Installed sfa to $out"
Write-Host "Run: sfa doctor"
