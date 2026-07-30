[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'

$schemaPath = '.\api\openapi\v1\openapi.yaml'
$guidePath = '.\docs\integrations\FRONTEND_INTEGRATION.md'
$schema = Get-Content $schemaPath -Raw
$guide = Get-Content $guidePath -Raw

$operationIDs = @(
    [regex]::Matches($schema, '(?m)^\s+operationId:\s+(\S+)\s*$') |
        ForEach-Object { $_.Groups[1].Value }
)
if ($operationIDs.Count -eq 0) {
    throw 'No OpenAPI operation IDs were discovered.'
}
$duplicates = @($operationIDs | Group-Object | Where-Object Count -gt 1)
if ($duplicates.Count -gt 0) {
    throw "Duplicate OpenAPI operation IDs: $($duplicates.Name -join ', ')"
}

$missingOperations = @($operationIDs | Where-Object { $guide -notmatch [regex]::Escape("``$_``") })
if ($missingOperations.Count -gt 0) {
    throw "Frontend guide is missing operation IDs: $($missingOperations -join ', ')"
}

$paths = @(
    [regex]::Matches($schema, '(?m)^  (/[^:]+):\s*$') |
        ForEach-Object { $_.Groups[1].Value }
)
$missingPaths = @($paths | Where-Object { -not $guide.Contains($_) })
if ($missingPaths.Count -gt 0) {
    throw "Frontend guide is missing API paths: $($missingPaths -join ', ')"
}

$requiredGuidance = @(
    'credentials: "include"',
    'X-CSRF-Token',
    'Idempotency-Key',
    'multipart/form-data',
    'must_change_password',
    'next_cursor',
    'invalid_responsible_member',
    'task_v2_required',
    'responsible_user_ids',
    'attachment_pending',
    'project_inactive',
    'content_path'
)
$missingGuidance = @($requiredGuidance | Where-Object { -not $guide.Contains($_) })
if ($missingGuidance.Count -gt 0) {
    throw "Frontend guide is missing required browser guidance: $($missingGuidance -join ', ')"
}

Write-Output "Frontend integration guide covers $($operationIDs.Count) OpenAPI operations and $($paths.Count) paths."
