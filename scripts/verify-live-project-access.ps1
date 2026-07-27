[CmdletBinding()]
param(
    [string]$EnvFile = '.env.local',
    [int]$Port = 18084
)

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

function New-RandomToken([int]$ByteCount) {
    return [Convert]::ToBase64String([Security.Cryptography.RandomNumberGenerator]::GetBytes($ByteCount)).TrimEnd('=').Replace('+', '-').Replace('/', '_')
}

function Invoke-JSON {
    param([string]$Uri, [string]$Method, $Body, $Session, [string]$CSRFToken)
    $parameters = @{ Uri = $Uri; Method = $Method; SkipHttpErrorCheck = $true }
    if ($null -ne $Body) {
        $parameters['ContentType'] = 'application/json'
        $parameters['Body'] = $Body | ConvertTo-Json -Compress
    }
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
if ($LASTEXITCODE -ne 0 -or [int]$userCount -ne 0) { throw "project verification requires a migrated database with zero users; found $userCount" }

$env:APP_ENV = 'test'
$env:HTTP_ADDR = "127.0.0.1:$Port"
$env:API_DOCS_ENABLED = 'false'
$env:SESSION_CSRF_KEY = [Convert]::ToBase64String([Security.Cryptography.RandomNumberGenerator]::GetBytes(32))
$bootstrapToken = New-RandomToken 32
$adminPassword = (New-RandomToken 24) + 'aA1!'
$env:BOOTSTRAP_TOKEN = $bootstrapToken
$runID = New-RandomToken 6
$process = Start-Process -FilePath '.\.local\bin\ppr.exe' -ArgumentList 'serve' -PassThru -WindowStyle Hidden -RedirectStandardOutput ".local\projects-$runID-stdout.log" -RedirectStandardError ".local\projects-$runID-stderr.log"
$baseURL = "http://127.0.0.1:$Port"

try {
    $started = $false
    foreach ($attempt in 1..50) {
        try { if ((Invoke-WebRequest -Uri "$baseURL/api/v1/health/ready" -SkipHttpErrorCheck -TimeoutSec 1).StatusCode -eq 200) { $started = $true; break } } catch {}
        Start-Sleep -Milliseconds 100
    }
    if (-not $started) { throw 'project verification server did not become ready' }

    $bootstrap = Invoke-JSON -Uri "$baseURL/api/v1/setup/bootstrap" -Method Post -Body @{ bootstrap_token = $bootstrapToken; username = 'admin'; email = 'admin@example.com'; password = $adminPassword }
    if ($bootstrap.StatusCode -ne 201) { throw "bootstrap returned $($bootstrap.StatusCode)" }
    $admin = Invoke-Login -BaseURL $baseURL -Identifier 'admin' -Password $adminPassword

    $createMember = Invoke-JSON -Uri "$baseURL/api/v1/admin/users" -Method Post -Body @{ username = 'member'; email = 'member@example.com'; role = 'member' } -Session $admin.Session -CSRFToken $admin.Response.csrf_token
    if ($createMember.StatusCode -ne 201) { throw "create Member returned $($createMember.StatusCode)" }
    $memberCredential = $createMember.Content | ConvertFrom-Json
    $memberID = $memberCredential.user.id
    $member = Invoke-Login -BaseURL $baseURL -Identifier 'member' -Password $memberCredential.temporary_password
    $memberPassword = (New-RandomToken 24) + 'aA1!'
    $change = Invoke-JSON -Uri "$baseURL/api/v1/auth/password" -Method Post -Body @{ password = $memberPassword } -Session $member.Session -CSRFToken $member.Response.csrf_token
    if ($change.StatusCode -ne 200) { throw "Member password change returned $($change.StatusCode)" }
    $member = Invoke-Login -BaseURL $baseURL -Identifier 'member' -Password $memberPassword

    $createProject = Invoke-JSON -Uri "$baseURL/api/v1/projects" -Method Post -Body @{ name = 'Site Alpha'; description_markdown = 'Internal **project**' } -Session $admin.Session -CSRFToken $admin.Response.csrf_token
    if ($createProject.StatusCode -ne 201) { throw "create project returned $($createProject.StatusCode)" }
    $project = ($createProject.Content | ConvertFrom-Json).project
    $projectID = $project.id

    $hiddenList = Invoke-WebRequest -Uri "$baseURL/api/v1/projects" -WebSession $member.Session -SkipHttpErrorCheck
    if ($hiddenList.StatusCode -ne 200 -or ($hiddenList.Content | ConvertFrom-Json).projects.Count -ne 0) { throw 'unassigned Member project list was not empty' }
    $hiddenRead = Invoke-WebRequest -Uri "$baseURL/api/v1/projects/$projectID" -WebSession $member.Session -SkipHttpErrorCheck
    if ($hiddenRead.StatusCode -ne 404) { throw "unassigned Member project read returned $($hiddenRead.StatusCode)" }

    $addMember = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/members/$memberID" -Method Put -Body $null -Session $admin.Session -CSRFToken $admin.Response.csrf_token
    if ($addMember.StatusCode -ne 201) { throw "add membership returned $($addMember.StatusCode)" }
    $visibleRead = Invoke-WebRequest -Uri "$baseURL/api/v1/projects/$projectID" -WebSession $member.Session -SkipHttpErrorCheck
    if ($visibleRead.StatusCode -ne 200) { throw "assigned Member project read returned $($visibleRead.StatusCode)" }

    $firstFence = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/geofence" -Method Put -Body @{ latitude = 22.5726; longitude = 88.3639; radius_metres = 250; max_accuracy_metres = 30; expected_version = 0 } -Session $admin.Session -CSRFToken $admin.Response.csrf_token
    if ($firstFence.StatusCode -ne 200 -or ($firstFence.Content | ConvertFrom-Json).geofence.version -ne 1) { throw "first geofence returned $($firstFence.StatusCode)" }
    $secondFence = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/geofence" -Method Put -Body @{ latitude = 22.5727; longitude = 88.3640; radius_metres = 275; max_accuracy_metres = 25; expected_version = 1 } -Session $admin.Session -CSRFToken $admin.Response.csrf_token
    if ($secondFence.StatusCode -ne 200 -or ($secondFence.Content | ConvertFrom-Json).geofence.version -ne 2) { throw "second geofence returned $($secondFence.StatusCode)" }
    $staleFence = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/geofence" -Method Put -Body @{ latitude = 22.57; longitude = 88.36; radius_metres = 100; max_accuracy_metres = 20; expected_version = 1 } -Session $admin.Session -CSRFToken $admin.Response.csrf_token
    if ($staleFence.StatusCode -ne 409) { throw "stale geofence returned $($staleFence.StatusCode)" }

    $removeMember = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/members/$memberID" -Method Delete -Body $null -Session $admin.Session -CSRFToken $admin.Response.csrf_token
    if ($removeMember.StatusCode -ne 204) { throw "remove membership returned $($removeMember.StatusCode)" }
    $revokedRead = Invoke-WebRequest -Uri "$baseURL/api/v1/projects/$projectID" -WebSession $member.Session -SkipHttpErrorCheck
    if ($revokedRead.StatusCode -ne 404) { throw "removed Member project read returned $($revokedRead.StatusCode)" }

    [pscustomobject]@{ ProjectCreate = 201; HiddenBeforeMembership = 404; MembershipAdd = 201; VisibleAfterMembership = 200; GeofenceV1 = 200; GeofenceV2 = 200; StaleGeofence = 409; MembershipRemove = 204; HiddenAfterRemoval = 404 }
}
finally {
    if (-not $process.HasExited) { Stop-Process -Id $process.Id; $process.WaitForExit() }
}

& psql $databaseURL -X -v ON_ERROR_STOP=1 -At -F ' | ' -c "SELECT (SELECT count(*) FROM public.project_members), (SELECT count(*) FROM public.project_geofences), (SELECT count(*) FROM public.audit_events WHERE action LIKE 'project.%' OR action LIKE 'authorization.project%');"
if ($LASTEXITCODE -ne 0) { throw 'project-access persistence inspection failed' }
