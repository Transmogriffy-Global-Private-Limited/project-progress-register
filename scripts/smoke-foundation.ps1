[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'

function Test-FoundationState {
    param(
        [Parameter(Mandatory)]
        [bool]$DocsEnabled,

        [Parameter(Mandatory)]
        [int]$Port
    )

    $env:APP_ENV = 'test'
    $env:HTTP_ADDR = "127.0.0.1:$Port"
    $env:DATABASE_URL = 'postgres://ppr_test:unused@127.0.0.1:1/ppr_test?sslmode=disable&connect_timeout=1'
    $env:API_DOCS_ENABLED = $DocsEnabled.ToString().ToLowerInvariant()
    $env:SESSION_CSRF_KEY = 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA='

    if (Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue) {
        throw "smoke-test port $Port is already in use"
    }

    $stdoutPath = Join-Path (Get-Location) ".local\smoke-$Port-stdout.log"
    $stderrPath = Join-Path (Get-Location) ".local\smoke-$Port-stderr.log"
    $process = Start-Process -FilePath '.\.local\bin\ppr.exe' -ArgumentList 'serve' -PassThru -WindowStyle Hidden -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath

    try {
        $started = $false
        foreach ($attempt in 1..30) {
            try {
                $response = Invoke-WebRequest -Uri "http://127.0.0.1:$Port/api/v1/health/live" -UseBasicParsing -SkipHttpErrorCheck -TimeoutSec 1
                if ($response.StatusCode -eq 200) {
                    $started = $true
                    break
                }
            }
            catch {
                # The child process may still be binding its loopback listener.
            }
            Start-Sleep -Milliseconds 100
        }
        if (-not $started) { throw "smoke server on port $Port did not start" }

        $homeResponse = Invoke-WebRequest -Uri "http://127.0.0.1:$Port/" -UseBasicParsing -SkipHttpErrorCheck
        $live = Invoke-WebRequest -Uri "http://127.0.0.1:$Port/api/v1/health/live" -UseBasicParsing -SkipHttpErrorCheck
        $ready = Invoke-WebRequest -Uri "http://127.0.0.1:$Port/api/v1/health/ready" -UseBasicParsing -SkipHttpErrorCheck
        $schema = Invoke-WebRequest -Uri "http://127.0.0.1:$Port/api/openapi/v1/openapi.yaml" -UseBasicParsing -SkipHttpErrorCheck
        $docs = Invoke-WebRequest -Uri "http://127.0.0.1:$Port/api/docs/" -UseBasicParsing -SkipHttpErrorCheck
        $listener = Get-NetTCPConnection -State Listen -LocalPort $Port | Select-Object -First 1

        if ($homeResponse.StatusCode -ne 200 -or $live.StatusCode -ne 200 -or $ready.StatusCode -ne 503) {
            throw "unexpected foundation status: home=$($homeResponse.StatusCode) live=$($live.StatusCode) ready=$($ready.StatusCode)"
        }
        $expectedDocsStatus = if ($DocsEnabled) { 200 } else { 404 }
        if ($schema.StatusCode -ne $expectedDocsStatus -or $docs.StatusCode -ne $expectedDocsStatus) {
            throw "unexpected docs status: schema=$($schema.StatusCode) docs=$($docs.StatusCode) expected=$expectedDocsStatus"
        }
        if ($listener.LocalAddress -ne '127.0.0.1') {
            throw "server listened on unexpected address $($listener.LocalAddress)"
        }
        if ($listener.OwningProcess -ne $process.Id) {
            throw "port $Port is owned by process $($listener.OwningProcess), expected $($process.Id)"
        }

        [pscustomobject]@{
            DocsEnabled = $DocsEnabled
            Home        = $homeResponse.StatusCode
            Live        = $live.StatusCode
            Ready       = $ready.StatusCode
            Schema      = $schema.StatusCode
            Docs        = $docs.StatusCode
            Listener    = $listener.LocalAddress
            Port        = $listener.LocalPort
        }
    }
    finally {
        if (-not $process.HasExited) {
            Stop-Process -Id $process.Id
            $process.WaitForExit()
        }
    }
}

.\scripts\build.ps1
Test-FoundationState -DocsEnabled $true -Port 18080
Test-FoundationState -DocsEnabled $false -Port 18081
