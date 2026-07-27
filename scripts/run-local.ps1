[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
if ([string]::IsNullOrWhiteSpace($env:DATABASE_URL)) {
    throw 'Set DATABASE_URL in this PowerShell session before running the application.'
}
if ([string]::IsNullOrWhiteSpace($env:HTTP_ADDR)) {
    $env:HTTP_ADDR = '127.0.0.1:8080'
}
if ([string]::IsNullOrWhiteSpace($env:APP_ENV)) {
    $env:APP_ENV = 'development'
}
& go run .\cmd\ppr serve
if ($LASTEXITCODE -ne 0) { throw 'application exited with an error' }
