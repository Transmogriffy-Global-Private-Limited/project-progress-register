[CmdletBinding()]
param(
    [string]$EnvFile = '.env.local',
    [int]$Port = 18082
)

$ErrorActionPreference = 'Stop'

function Import-LocalEnvironment {
    param([Parameter(Mandatory)][string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) {
        throw "environment file not found: $Path"
    }
    foreach ($rawLine in Get-Content -LiteralPath $Path) {
        $line = $rawLine.Trim()
        if (-not $line -or $line.StartsWith('#')) { continue }
        $parts = $line -split '=', 2
        if ($parts.Count -ne 2) { throw "invalid environment line for $($parts[0])" }
        $value = $parts[1].Trim()
        if (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'"))) {
            $value = $value.Substring(1, $value.Length - 2)
        }
        Set-Item -Path "Env:$($parts[0].Trim())" -Value $value
    }
}

function New-RandomToken {
    param([Parameter(Mandatory)][int]$ByteCount)
    $bytes = [Security.Cryptography.RandomNumberGenerator]::GetBytes($ByteCount)
    return [Convert]::ToBase64String($bytes).TrimEnd('=').Replace('+', '-').Replace('/', '_')
}

Import-LocalEnvironment -Path $EnvFile
if ([string]::IsNullOrWhiteSpace($env:DATABASE_URL)) { throw 'DATABASE_URL is required' }
if (Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue) {
    throw "verification port $Port is already in use"
}

$databaseURL = if ([string]::IsNullOrWhiteSpace($env:MIGRATION_DATABASE_URL)) { $env:DATABASE_URL } else { $env:MIGRATION_DATABASE_URL }
$userCount = & psql $databaseURL -X -v ON_ERROR_STOP=1 -Atc 'SELECT count(*) FROM public.users'
if ($LASTEXITCODE -ne 0) { throw 'could not inspect identity verification state' }
if ([int]$userCount -ne 0) { throw "identity verification requires zero users; found $userCount" }

$env:APP_ENV = 'test'
$env:HTTP_ADDR = "127.0.0.1:$Port"
$env:API_DOCS_ENABLED = 'false'
$env:SESSION_CSRF_KEY = [Convert]::ToBase64String([Security.Cryptography.RandomNumberGenerator]::GetBytes(32))
$bootstrapToken = New-RandomToken -ByteCount 32
$password = (New-RandomToken -ByteCount 24) + 'aA1!'
$env:BOOTSTRAP_TOKEN = $bootstrapToken

$runID = New-RandomToken -ByteCount 6
$stdoutPath = Join-Path (Get-Location) ".local\identity-$runID-stdout.log"
$stderrPath = Join-Path (Get-Location) ".local\identity-$runID-stderr.log"
$process = Start-Process -FilePath '.\.local\bin\ppr.exe' -ArgumentList 'serve' -PassThru -WindowStyle Hidden -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath
$baseURL = "http://127.0.0.1:$Port"

try {
    $started = $false
    foreach ($attempt in 1..50) {
        try {
            $live = Invoke-WebRequest -Uri "$baseURL/api/v1/health/live" -SkipHttpErrorCheck -TimeoutSec 1
            if ($live.StatusCode -eq 200) { $started = $true; break }
        }
        catch {
            # The process may still be binding its loopback listener.
        }
        Start-Sleep -Milliseconds 100
    }
    if (-not $started) { throw 'identity verification server did not start' }

    $ready = Invoke-WebRequest -Uri "$baseURL/api/v1/health/ready" -SkipHttpErrorCheck
    if ($ready.StatusCode -ne 200) { throw "readiness status was $($ready.StatusCode)" }
    $anonymous = Invoke-WebRequest -Uri "$baseURL/api/v1/auth/session" -SkipHttpErrorCheck
    if ($anonymous.StatusCode -ne 401) { throw "anonymous session status was $($anonymous.StatusCode)" }

    $invalidBootstrap = @{
        bootstrap_token = 'definitely-invalid-bootstrap-token'
        username = 'invalidadmin'
        email = 'invalid@example.com'
        password = $password
    } | ConvertTo-Json -Compress
    $invalid = Invoke-WebRequest -Uri "$baseURL/api/v1/setup/bootstrap" -Method Post -ContentType 'application/json' -Body $invalidBootstrap -SkipHttpErrorCheck
    if ($invalid.StatusCode -ne 403) { throw "invalid bootstrap status was $($invalid.StatusCode)" }

    $bodyA = @{ bootstrap_token = $bootstrapToken; username = 'verifyadmina'; email = 'verify-a@example.com'; password = $password } | ConvertTo-Json -Compress
    $bodyB = @{ bootstrap_token = $bootstrapToken; username = 'verifyadminb'; email = 'verify-b@example.com'; password = $password } | ConvertTo-Json -Compress
    $client = [Net.Http.HttpClient]::new()
    try {
        $contentA = [Net.Http.StringContent]::new($bodyA, [Text.Encoding]::UTF8, 'application/json')
        $contentB = [Net.Http.StringContent]::new($bodyB, [Text.Encoding]::UTF8, 'application/json')
        $taskA = $client.PostAsync("$baseURL/api/v1/setup/bootstrap", $contentA)
        $taskB = $client.PostAsync("$baseURL/api/v1/setup/bootstrap", $contentB)
        $responseA = $taskA.GetAwaiter().GetResult()
        $responseB = $taskB.GetAwaiter().GetResult()
        $statusA = [int]$responseA.StatusCode
        $statusB = [int]$responseB.StatusCode
    }
    finally {
        $client.Dispose()
    }
    if ((@($statusA, $statusB) | Sort-Object) -join ',' -ne '201,404') {
        throw "concurrent bootstrap statuses were $statusA,$statusB"
    }
    $adminUsername = if ($statusA -eq 201) { 'verifyadmina' } else { 'verifyadminb' }
    $repeatBody = if ($statusA -eq 201) { $bodyA } else { $bodyB }
    $repeat = Invoke-WebRequest -Uri "$baseURL/api/v1/setup/bootstrap" -Method Post -ContentType 'application/json' -Body $repeatBody -SkipHttpErrorCheck
    if ($repeat.StatusCode -ne 404) { throw "repeat bootstrap status was $($repeat.StatusCode)" }

    $wrongBody = @{ identifier = $adminUsername; password = 'wrong-password-value' } | ConvertTo-Json -Compress
    $wrong = Invoke-WebRequest -Uri "$baseURL/api/v1/auth/login" -Method Post -ContentType 'application/json' -Body $wrongBody -SkipHttpErrorCheck
    if ($wrong.StatusCode -ne 401) { throw "wrong login status was $($wrong.StatusCode)" }
    $loginBody = @{ identifier = $adminUsername; password = $password } | ConvertTo-Json -Compress
    $login = Invoke-WebRequest -Uri "$baseURL/api/v1/auth/login" -Method Post -ContentType 'application/json' -Body $loginBody -SessionVariable browserSession -SkipHttpErrorCheck
    if ($login.StatusCode -ne 200) { throw "login status was $($login.StatusCode)" }
    $setCookie = [string]$login.Headers['Set-Cookie']
    if ($setCookie -notmatch 'HttpOnly' -or $setCookie -notmatch 'SameSite=Lax') { throw 'session cookie security attributes were incomplete' }
    $loginPayload = $login.Content | ConvertFrom-Json
    if ([string]::IsNullOrWhiteSpace($loginPayload.csrf_token)) { throw 'login response omitted CSRF token' }

    $current = Invoke-WebRequest -Uri "$baseURL/api/v1/auth/session" -WebSession $browserSession -SkipHttpErrorCheck
    if ($current.StatusCode -ne 200) { throw "current session status was $($current.StatusCode)" }
    foreach ($attempt in 1..6) {
        $throttleBody = @{ identifier = 'missing-verification-user'; password = 'wrong-password-value' } | ConvertTo-Json -Compress
        $throttled = Invoke-WebRequest -Uri "$baseURL/api/v1/auth/login" -Method Post -ContentType 'application/json' -Body $throttleBody -SkipHttpErrorCheck
        if ($throttled.StatusCode -ne 401) { throw "throttle attempt $attempt status was $($throttled.StatusCode)" }
    }

    $missingCSRF = Invoke-WebRequest -Uri "$baseURL/api/v1/auth/logout" -Method Post -WebSession $browserSession -SkipHttpErrorCheck
    if ($missingCSRF.StatusCode -ne 403) { throw "missing CSRF logout status was $($missingCSRF.StatusCode)" }
    $logout = Invoke-WebRequest -Uri "$baseURL/api/v1/auth/logout" -Method Post -Headers @{ 'X-CSRF-Token' = $loginPayload.csrf_token } -WebSession $browserSession -SkipHttpErrorCheck
    if ($logout.StatusCode -ne 200) { throw "logout status was $($logout.StatusCode)" }
    $afterLogout = Invoke-WebRequest -Uri "$baseURL/api/v1/auth/session" -WebSession $browserSession -SkipHttpErrorCheck
    if ($afterLogout.StatusCode -ne 401) { throw "post-logout session status was $($afterLogout.StatusCode)" }

    [pscustomobject]@{
        Ready = $ready.StatusCode
        AnonymousSession = $anonymous.StatusCode
        InvalidBootstrap = $invalid.StatusCode
        ConcurrentBootstrap = "$statusA,$statusB"
        RepeatBootstrap = $repeat.StatusCode
        WrongLogin = $wrong.StatusCode
        Login = $login.StatusCode
        CurrentSession = $current.StatusCode
        Throttle = '6 generic 401 responses'
        MissingCSRF = $missingCSRF.StatusCode
        Logout = $logout.StatusCode
        AfterLogout = $afterLogout.StatusCode
        Cookie = 'HttpOnly; SameSite=Lax'
    }
}
finally {
    if (-not $process.HasExited) {
        Stop-Process -Id $process.Id
        $process.WaitForExit()
    }
}

$counts = & psql $databaseURL -X -v ON_ERROR_STOP=1 -At -F ' | ' -c "
SELECT 'users', count(*)::text FROM public.users
UNION ALL SELECT 'sessions', count(*)::text FROM public.sessions
UNION ALL SELECT 'revoked_sessions', count(*)::text FROM public.sessions WHERE revoked_at IS NOT NULL
UNION ALL SELECT 'throttle_rows', count(*)::text FROM public.login_throttles
UNION ALL SELECT 'audit_events', count(*)::text FROM public.audit_events;
SELECT action, count(*) FROM public.audit_events GROUP BY action ORDER BY action;"
if ($LASTEXITCODE -ne 0) { throw 'identity persistence inspection failed' }
$counts
