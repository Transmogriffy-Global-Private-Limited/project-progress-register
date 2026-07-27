[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [ValidateSet('up', 'status')]
    [string]$Command
)

$ErrorActionPreference = 'Stop'
if ([string]::IsNullOrWhiteSpace($env:DATABASE_URL)) {
    throw 'Set DATABASE_URL in this PowerShell session before using migrations.'
}
& go run .\cmd\ppr migrate $Command
if ($LASTEXITCODE -ne 0) { throw "migration command '$Command' failed" }
