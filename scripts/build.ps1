[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
New-Item -ItemType Directory -Force -Path '.\.local\bin' | Out-Null
& go build -o '.\.local\bin\ppr.exe' .\cmd\ppr
if ($LASTEXITCODE -ne 0) { throw 'go build failed' }
