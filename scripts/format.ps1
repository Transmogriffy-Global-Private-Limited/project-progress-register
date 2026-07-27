[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$goFiles = @(Get-ChildItem .\cmd, .\internal, .\api -Recurse -File -Filter '*.go' | ForEach-Object { $_.FullName })
if ($goFiles.Count -gt 0) {
    & gofmt -w @goFiles
    if ($LASTEXITCODE -ne 0) { throw 'gofmt failed' }
}
