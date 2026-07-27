[CmdletBinding()]
param(
    [string]$EnvFile = '.env.local',
    [int]$Port = 18083
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
    $parameters = @{ Uri = $Uri; Method = $Method; ContentType = 'application/json'; Body = ($Body | ConvertTo-Json -Compress); SkipHttpErrorCheck = $true }
    if ($Session) { $parameters['WebSession'] = $Session }
    if ($CSRFToken) { $parameters['Headers'] = @{ 'X-CSRF-Token' = $CSRFToken } }
    return Invoke-WebRequest @parameters
}

function Invoke-Login([string]$BaseURL, [string]$Identifier, [string]$Password) {
    $body = @{ identifier = $Identifier; password = $Password } | ConvertTo-Json -Compress
    $response = Invoke-WebRequest -Uri "$BaseURL/api/v1/auth/login" -Method Post -ContentType 'application/json' -Body $body -SessionVariable loginSession -SkipHttpErrorCheck
    if ($response.StatusCode -ne 200) { throw "login for $Identifier returned $($response.StatusCode)" }
    return @{ Response = $response.Content | ConvertFrom-Json; Session = $loginSession }
}

Import-EnvironmentFile -Path $EnvFile
if ([string]::IsNullOrWhiteSpace($env:DATABASE_URL)) { throw 'DATABASE_URL is required' }
if (Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue) { throw "verification port $Port is already in use" }
$databaseURL = if ([string]::IsNullOrWhiteSpace($env:MIGRATION_DATABASE_URL)) { $env:DATABASE_URL } else { $env:MIGRATION_DATABASE_URL }
$userCount = & psql $databaseURL -X -v ON_ERROR_STOP=1 -Atc 'SELECT count(*) FROM public.users'
if ($LASTEXITCODE -ne 0 -or [int]$userCount -ne 0) { throw "account verification requires a migrated database with zero users; found $userCount" }

$env:APP_ENV = 'test'
$env:HTTP_ADDR = "127.0.0.1:$Port"
$env:API_DOCS_ENABLED = 'false'
$env:SESSION_CSRF_KEY = [Convert]::ToBase64String([Security.Cryptography.RandomNumberGenerator]::GetBytes(32))
$bootstrapToken = New-RandomToken 32
$adminPassword = (New-RandomToken 24) + 'aA1!'
$env:BOOTSTRAP_TOKEN = $bootstrapToken
$runID = New-RandomToken 6
$process = Start-Process -FilePath '.\.local\bin\ppr.exe' -ArgumentList 'serve' -PassThru -WindowStyle Hidden -RedirectStandardOutput ".local\account-$runID-stdout.log" -RedirectStandardError ".local\account-$runID-stderr.log"
$baseURL = "http://127.0.0.1:$Port"

try {
    $started = $false
    foreach ($attempt in 1..50) {
        try { if ((Invoke-WebRequest -Uri "$baseURL/api/v1/health/ready" -SkipHttpErrorCheck -TimeoutSec 1).StatusCode -eq 200) { $started = $true; break } } catch {}
        Start-Sleep -Milliseconds 100
    }
    if (-not $started) { throw 'account verification server did not become ready' }

    $bootstrap = Invoke-JSON -Uri "$baseURL/api/v1/setup/bootstrap" -Method Post -Body @{ bootstrap_token = $bootstrapToken; username = 'admin'; email = 'admin@example.com'; password = $adminPassword } -Session $null -CSRFToken ''
    if ($bootstrap.StatusCode -ne 201) { throw "bootstrap returned $($bootstrap.StatusCode)" }
    $adminLogin = Invoke-Login -BaseURL $baseURL -Identifier 'admin' -Password $adminPassword

    $create = Invoke-JSON -Uri "$baseURL/api/v1/admin/users" -Method Post -Body @{ username = 'member'; email = 'member@example.com'; role = 'member' } -Session $adminLogin.Session -CSRFToken $adminLogin.Response.csrf_token
    if ($create.StatusCode -ne 201 -or $create.Headers['Cache-Control'] -ne 'no-store') { throw "create user returned $($create.StatusCode)" }
    $created = $create.Content | ConvertFrom-Json
    $memberID = $created.user.id
    $temporaryPassword = $created.temporary_password

    $memberLogin = Invoke-Login -BaseURL $baseURL -Identifier 'member' -Password $temporaryPassword
    if (-not $memberLogin.Response.user.must_change_password) { throw 'created user did not require a password change' }
    $denied = Invoke-WebRequest -Uri "$baseURL/api/v1/admin/users" -WebSession $memberLogin.Session -SkipHttpErrorCheck
    if ($denied.StatusCode -ne 403) { throw "Member Admin-list status was $($denied.StatusCode)" }

    $memberPassword = (New-RandomToken 24) + 'aA1!'
    $changed = Invoke-JSON -Uri "$baseURL/api/v1/auth/password" -Method Post -Body @{ password = $memberPassword } -Session $memberLogin.Session -CSRFToken $memberLogin.Response.csrf_token
    if ($changed.StatusCode -ne 200) { throw "password change returned $($changed.StatusCode)" }
    $memberLogin = Invoke-Login -BaseURL $baseURL -Identifier 'member' -Password $memberPassword
    if ($memberLogin.Response.user.must_change_password) { throw 'password replacement did not clear forced-change state' }

    $reset = Invoke-JSON -Uri "$baseURL/api/v1/admin/users/$memberID/password-reset" -Method Post -Body @{} -Session $adminLogin.Session -CSRFToken $adminLogin.Response.csrf_token
    if ($reset.StatusCode -ne 200 -or $reset.Headers['Cache-Control'] -ne 'no-store') { throw "password reset returned $($reset.StatusCode)" }
    $resetPayload = $reset.Content | ConvertFrom-Json
    $staleSession = Invoke-WebRequest -Uri "$baseURL/api/v1/auth/session" -WebSession $memberLogin.Session -SkipHttpErrorCheck
    if ($staleSession.StatusCode -ne 401) { throw 'password reset did not revoke the Member session' }

    $memberLogin = Invoke-Login -BaseURL $baseURL -Identifier 'member' -Password $resetPayload.temporary_password
    $memberPassword = (New-RandomToken 24) + 'aA1!'
    $null = Invoke-JSON -Uri "$baseURL/api/v1/auth/password" -Method Post -Body @{ password = $memberPassword } -Session $memberLogin.Session -CSRFToken $memberLogin.Response.csrf_token

    $list = Invoke-WebRequest -Uri "$baseURL/api/v1/admin/users" -WebSession $adminLogin.Session -SkipHttpErrorCheck
    $member = (($list.Content | ConvertFrom-Json).users | Where-Object id -eq $memberID)
    $promote = Invoke-JSON -Uri "$baseURL/api/v1/admin/users/$memberID" -Method Patch -Body @{ role = 'admin'; enabled = $true; expected_version = $member.version } -Session $adminLogin.Session -CSRFToken $adminLogin.Response.csrf_token
    if ($promote.StatusCode -ne 200) { throw "promotion returned $($promote.StatusCode)" }
    $promotedLogin = Invoke-Login -BaseURL $baseURL -Identifier 'member' -Password $memberPassword

    $list = Invoke-WebRequest -Uri "$baseURL/api/v1/admin/users" -WebSession $promotedLogin.Session -SkipHttpErrorCheck
    $users = ($list.Content | ConvertFrom-Json).users
    $originalAdmin = $users | Where-Object username -eq 'admin'
    $demoteOriginal = Invoke-JSON -Uri "$baseURL/api/v1/admin/users/$($originalAdmin.id)" -Method Patch -Body @{ role = 'member'; enabled = $true; expected_version = $originalAdmin.version } -Session $promotedLogin.Session -CSRFToken $promotedLogin.Response.csrf_token
    if ($demoteOriginal.StatusCode -ne 200) { throw "original Admin demotion returned $($demoteOriginal.StatusCode)" }

    $listAfterDemotion = Invoke-WebRequest -Uri "$baseURL/api/v1/admin/users" -WebSession $promotedLogin.Session
    $promoted = (($listAfterDemotion.Content | ConvertFrom-Json).users | Where-Object id -eq $memberID)
    $lastAdmin = Invoke-JSON -Uri "$baseURL/api/v1/admin/users/$memberID" -Method Patch -Body @{ role = 'member'; enabled = $true; expected_version = $promoted.version } -Session $promotedLogin.Session -CSRFToken $promotedLogin.Response.csrf_token
    if ($lastAdmin.StatusCode -ne 409 -or ($lastAdmin.Content | ConvertFrom-Json).error.code -ne 'last_admin') { throw "final Admin guard returned $($lastAdmin.StatusCode)" }

    $auditView = Invoke-WebRequest -Uri "$baseURL/api/v1/admin/audit/identity" -WebSession $promotedLogin.Session -SkipHttpErrorCheck
    if ($auditView.StatusCode -ne 200 -or ($auditView.Content | ConvertFrom-Json).audit_events.Count -lt 1) { throw "identity audit view returned $($auditView.StatusCode)" }

    [pscustomobject]@{ Bootstrap = 201; UserCreate = 201; ForcedPasswordChange = 200; MemberAdminDenied = 403; PasswordReset = 200; SessionRevoked = 401; Promotion = 200; SafeDemotion = 200; FinalAdminGuard = 409; AuditView = 200 }
}
finally {
    if (-not $process.HasExited) { Stop-Process -Id $process.Id; $process.WaitForExit() }
}

& psql $databaseURL -X -v ON_ERROR_STOP=1 -At -F ' | ' -c "SELECT action, count(*) FROM public.audit_events WHERE action IN ('identity.user_created','identity.user_updated','identity.password_reset','identity.password_changed','authorization.admin_users_denied') GROUP BY action ORDER BY action;"
if ($LASTEXITCODE -ne 0) { throw 'account-administration audit inspection failed' }
