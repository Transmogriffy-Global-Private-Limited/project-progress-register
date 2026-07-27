[CmdletBinding()]
param([string]$EnvFile = '.env.local', [int]$Port = 18086)

$ErrorActionPreference = 'Stop'

function Import-EnvironmentFile([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) { throw "environment file not found: $Path" }
    foreach ($rawLine in Get-Content -LiteralPath $Path) {
        $line = $rawLine.Trim()
        if (-not $line -or $line.StartsWith('#')) { continue }
        $parts = $line -split '=', 2
        if ($parts.Count -ne 2) { throw "invalid environment line for $($parts[0])" }
        $value = $parts[1].Trim()
        if (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'"))) { $value = $value.Substring(1, $value.Length - 2) }
        Set-Item -Path "Env:$($parts[0].Trim())" -Value $value
    }
}

function New-RandomToken([int]$ByteCount) { return [Convert]::ToBase64String([Security.Cryptography.RandomNumberGenerator]::GetBytes($ByteCount)).TrimEnd('=').Replace('+', '-').Replace('/', '_') }
function Invoke-JSON([string]$Uri, [string]$Method, $Body, $Session, [string]$CSRFToken) {
    $parameters = @{ Uri = $Uri; Method = $Method; SkipHttpErrorCheck = $true }
    if ($null -ne $Body) { $parameters['ContentType'] = 'application/json'; $parameters['Body'] = $Body | ConvertTo-Json -Depth 8 -Compress }
    if ($Session) { $parameters['WebSession'] = $Session }
    if ($CSRFToken) { $parameters['Headers'] = @{ 'X-CSRF-Token' = $CSRFToken } }
    return Invoke-WebRequest @parameters
}
function Invoke-Login([string]$BaseURL, [string]$Identifier, [string]$Password) {
    $response = Invoke-WebRequest -Uri "$BaseURL/api/v1/auth/login" -Method Post -ContentType 'application/json' -Body (@{ identifier = $Identifier; password = $Password } | ConvertTo-Json -Compress) -SessionVariable loginSession -SkipHttpErrorCheck
    if ($response.StatusCode -ne 200) { throw "login for $Identifier returned $($response.StatusCode)" }
    return @{ Response = $response.Content | ConvertFrom-Json; Session = $loginSession }
}

Import-EnvironmentFile -Path $EnvFile
if ([string]::IsNullOrWhiteSpace($env:DATABASE_URL)) { throw 'DATABASE_URL is required' }
if (Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue) { throw "verification port $Port is already in use" }
$databaseURL = if ([string]::IsNullOrWhiteSpace($env:MIGRATION_DATABASE_URL)) { $env:DATABASE_URL } else { $env:MIGRATION_DATABASE_URL }
$userCount = & psql $databaseURL -X -v ON_ERROR_STOP=1 -Atc 'SELECT count(*) FROM public.users'
if ($LASTEXITCODE -ne 0 -or [int]$userCount -ne 0) { throw "progress verification requires a fully migrated database with zero users; found $userCount" }

$runID = New-RandomToken 6
$storageRoot = Join-Path (Get-Location) ".local\progress-live-$runID"
$photoPath = Join-Path (Get-Location) ".local\progress-photo-$runID.jpg"
$documentPath = Join-Path (Get-Location) ".local\progress-document-$runID.pdf"
[IO.File]::WriteAllBytes($photoPath, [byte[]](0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01, 0xFF, 0xD9))
[IO.File]::WriteAllText($documentPath, "%PDF-1.7`nprogress verifier")

$env:APP_ENV = 'test'; $env:HTTP_ADDR = "127.0.0.1:$Port"; $env:API_DOCS_ENABLED = 'false'
$env:SESSION_CSRF_KEY = [Convert]::ToBase64String([Security.Cryptography.RandomNumberGenerator]::GetBytes(32))
$env:BOOTSTRAP_TOKEN = New-RandomToken 32; $env:ATTACHMENT_STORAGE_DIR = $storageRoot
$process = Start-Process -FilePath '.\.local\bin\ppr.exe' -ArgumentList 'serve' -PassThru -WindowStyle Hidden -RedirectStandardOutput ".local\progress-$runID-stdout.log" -RedirectStandardError ".local\progress-$runID-stderr.log"
$baseURL = "http://127.0.0.1:$Port"

try {
    $started = $false
    foreach ($attempt in 1..50) { try { if ((Invoke-WebRequest -Uri "$baseURL/api/v1/health/ready" -SkipHttpErrorCheck -TimeoutSec 1).StatusCode -eq 200) { $started = $true; break } } catch {}; Start-Sleep -Milliseconds 100 }
    if (-not $started) { throw 'progress verification server did not become ready' }

    $adminPassword = (New-RandomToken 24) + 'aA1!'
    $bootstrap = Invoke-JSON -Uri "$baseURL/api/v1/setup/bootstrap" -Method Post -Body @{ bootstrap_token = $env:BOOTSTRAP_TOKEN; username = 'admin'; email = 'admin@example.com'; password = $adminPassword }
    if ($bootstrap.StatusCode -ne 201) { throw "bootstrap returned $($bootstrap.StatusCode)" }
    $admin = Invoke-Login -BaseURL $baseURL -Identifier 'admin' -Password $adminPassword
    $createMember = Invoke-JSON -Uri "$baseURL/api/v1/admin/users" -Method Post -Body @{ username = 'member'; email = 'member@example.com'; role = 'member' } -Session $admin.Session -CSRFToken $admin.Response.csrf_token
    $credential = $createMember.Content | ConvertFrom-Json; $firstLogin = Invoke-Login -BaseURL $baseURL -Identifier 'member' -Password $credential.temporary_password
    $memberPassword = (New-RandomToken 24) + 'aA1!'; [void](Invoke-JSON -Uri "$baseURL/api/v1/auth/password" -Method Post -Body @{ password = $memberPassword } -Session $firstLogin.Session -CSRFToken $firstLogin.Response.csrf_token)
    $member = Invoke-Login -BaseURL $baseURL -Identifier 'member' -Password $memberPassword

    $projectResponse = Invoke-JSON -Uri "$baseURL/api/v1/projects" -Method Post -Body @{ name = 'Site Alpha'; description_markdown = 'Site' } -Session $admin.Session -CSRFToken $admin.Response.csrf_token
    $project = ($projectResponse.Content | ConvertFrom-Json).project; $projectID = $project.id
    [void](Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/members/$($credential.user.id)" -Method Put -Body $null -Session $admin.Session -CSRFToken $admin.Response.csrf_token)
    [void](Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/geofence" -Method Put -Body @{ latitude = 22.5726; longitude = 88.3639; radius_metres = 100; max_accuracy_metres = 20; expected_version = 0 } -Session $admin.Session -CSRFToken $admin.Response.csrf_token)
    $taskResponse = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/tasks" -Method Post -Body @{ name = 'Foundation'; goals_markdown = ''; description_markdown = ''; responsible_user_id = $null; target_date = $null } -Session $member.Session -CSRFToken $member.Response.csrf_token
    $taskID = ($taskResponse.Content | ConvertFrom-Json).task.id

    $metadata = @{ content_markdown = 'Completed **foundation**'; location = @{ latitude = 22.5726; longitude = 88.3639; accuracy_metres = 5; browser_observed_at = (Get-Date).ToUniversalTime().ToString('o') }; location_unavailable_reason = $null; attachments = @(@{ source = 'camera'; browser_last_modified_at = $null }, @{ source = 'upload'; browser_last_modified_at = $null }) } | ConvertTo-Json -Depth 8 -Compress
    $create = Invoke-WebRequest -Uri "$baseURL/api/v1/projects/$projectID/tasks/$taskID/progress-updates" -Method Post -Form @{ metadata = $metadata; files = @((Get-Item -LiteralPath $photoPath), (Get-Item -LiteralPath $documentPath)) } -Headers @{ 'X-CSRF-Token' = $member.Response.csrf_token; 'Idempotency-Key' = "progress-$runID-0001" } -WebSession $member.Session -SkipHttpErrorCheck
    if ($create.StatusCode -ne 201) { throw "progress create returned $($create.StatusCode) $($create.Content)" }
    $update = ($create.Content | ConvertFrom-Json).progress_update
    if ($update.attachments.Count -ne 2 -or $update.attachments[0].verification_status -ne 'verified' -or $update.attachments[1].verification_status -ne 'non_verified') { throw 'per-file verification classification was incorrect' }
    if ($update.evidence.reported_location.latitude -ne 22.5726 -or $update.evidence.location_status -ne 'verified') { throw 'upload geotag was not preserved' }

    $download = Invoke-WebRequest -Uri "$baseURL$($update.attachments[1].content_path)" -WebSession $member.Session -SkipHttpErrorCheck
    if ($download.StatusCode -ne 200 -or $download.Headers.'Content-Disposition' -notmatch 'attachment') { throw "attachment download returned $($download.StatusCode)" }
    $edit = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/tasks/$taskID/progress-updates/$($update.id)" -Method Patch -Body @{ content_markdown = 'Revised progress'; expected_version = 1 } -Session $member.Session -CSRFToken $member.Response.csrf_token
    if ($edit.StatusCode -ne 200 -or ($edit.Content | ConvertFrom-Json).progress_update.revisions.Count -ne 1) { throw "progress edit returned $($edit.StatusCode)" }

    [pscustomobject]@{ Create = 201; CameraPhoto = 'verified'; UploadedDocument = 'non_verified'; Geotagged = $true; Download = 200; Revision = 1; StorageRoot = $storageRoot }
}
finally {
    if (-not $process.HasExited) { Stop-Process -Id $process.Id; $process.WaitForExit() }
}

& psql $databaseURL -X -v ON_ERROR_STOP=1 -At -F ' | ' -c "SELECT (SELECT count(*) FROM public.progress_updates), (SELECT count(*) FROM public.progress_update_revisions), (SELECT count(*) FROM public.progress_attachments WHERE storage_state='available'), (SELECT count(*) FROM public.audit_events WHERE action IN ('progress.created','progress.updated','attachment.available','attachment.downloaded'));"
if ($LASTEXITCODE -ne 0) { throw 'progress persistence inspection failed' }
