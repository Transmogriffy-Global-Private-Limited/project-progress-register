[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'

$scriptErrors = @()
foreach ($script in Get-ChildItem .\scripts -File -Filter '*.ps1') {
    $tokens = $null
    $errors = $null
    [void][System.Management.Automation.Language.Parser]::ParseFile($script.FullName, [ref]$tokens, [ref]$errors)
    if ($errors.Count -gt 0) {
        $scriptErrors += $errors | ForEach-Object { "$($script.Name): $($_.Message)" }
    }
}
if ($scriptErrors.Count -gt 0) {
    throw "PowerShell syntax errors:`n$($scriptErrors -join "`n")"
}

$goFiles = @(Get-ChildItem .\cmd, .\internal, .\api -Recurse -File -Filter '*.go' | ForEach-Object { $_.FullName })
$unformatted = @()
if ($goFiles.Count -gt 0) {
    $unformatted = @(& gofmt -l @goFiles)
    if ($LASTEXITCODE -ne 0) { throw 'gofmt check failed' }
}
if ($unformatted.Count -gt 0) {
    throw "Go files require formatting:`n$($unformatted -join "`n")"
}

& go mod tidy -diff
if ($LASTEXITCODE -ne 0) { throw 'go.mod or go.sum is not tidy' }

& go vet .\...
if ($LASTEXITCODE -ne 0) { throw 'go vet failed' }

& go test .\...
if ($LASTEXITCODE -ne 0) { throw 'go test failed' }

New-Item -ItemType Directory -Force -Path '.\.local\bin' | Out-Null
& go build -o '.\.local\bin\ppr.exe' .\cmd\ppr
if ($LASTEXITCODE -ne 0) { throw 'go build failed' }

Write-Output 'Full verification passed.'
