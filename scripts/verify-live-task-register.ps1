[CmdletBinding()]
param(
    [string]$EnvFile = '.env.local',
    [int]$Port = 18085
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

function New-ReadyMember([string]$BaseURL, [string]$Username, $Admin) {
    $create = Invoke-JSON -Uri "$BaseURL/api/v1/admin/users" -Method Post -Body @{ username = $Username; email = "$Username@example.com"; role = 'member' } -Session $Admin.Session -CSRFToken $Admin.Response.csrf_token
    if ($create.StatusCode -ne 201) { throw "create $Username returned $($create.StatusCode)" }
    $credential = $create.Content | ConvertFrom-Json
    $login = Invoke-Login -BaseURL $BaseURL -Identifier $Username -Password $credential.temporary_password
    $password = (New-RandomToken 24) + 'aA1!'
    $change = Invoke-JSON -Uri "$BaseURL/api/v1/auth/password" -Method Post -Body @{ password = $password } -Session $login.Session -CSRFToken $login.Response.csrf_token
    if ($change.StatusCode -ne 200) { throw "password change for $Username returned $($change.StatusCode)" }
    $ready = Invoke-Login -BaseURL $BaseURL -Identifier $Username -Password $password
    return @{ ID = $credential.user.id; Session = $ready.Session; Response = $ready.Response }
}

Import-EnvironmentFile -Path $EnvFile
if ([string]::IsNullOrWhiteSpace($env:DATABASE_URL)) { throw 'DATABASE_URL is required' }
if (Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue) { throw "verification port $Port is already in use" }
$databaseURL = if ([string]::IsNullOrWhiteSpace($env:MIGRATION_DATABASE_URL)) { $env:DATABASE_URL } else { $env:MIGRATION_DATABASE_URL }
$userCount = & psql $databaseURL -X -v ON_ERROR_STOP=1 -Atc 'SELECT count(*) FROM public.users'
if ($LASTEXITCODE -ne 0 -or [int]$userCount -ne 0) { throw "task verification requires a fully migrated database with zero users; found $userCount" }

$env:APP_ENV = 'test'
$env:HTTP_ADDR = "127.0.0.1:$Port"
$env:API_DOCS_ENABLED = 'false'
$env:SESSION_CSRF_KEY = [Convert]::ToBase64String([Security.Cryptography.RandomNumberGenerator]::GetBytes(32))
$bootstrapToken = New-RandomToken 32
$adminPassword = (New-RandomToken 24) + 'aA1!'
$env:BOOTSTRAP_TOKEN = $bootstrapToken
$runID = New-RandomToken 6
$process = Start-Process -FilePath '.\.local\bin\ppr.exe' -ArgumentList 'serve' -PassThru -WindowStyle Hidden -RedirectStandardOutput ".local\tasks-$runID-stdout.log" -RedirectStandardError ".local\tasks-$runID-stderr.log"
$baseURL = "http://127.0.0.1:$Port"

try {
    $started = $false
    foreach ($attempt in 1..50) {
        try { if ((Invoke-WebRequest -Uri "$baseURL/api/v1/health/ready" -SkipHttpErrorCheck -TimeoutSec 1).StatusCode -eq 200) { $started = $true; break } } catch {}
        Start-Sleep -Milliseconds 100
    }
    if (-not $started) { throw 'task verification server did not become ready' }

    $bootstrap = Invoke-JSON -Uri "$baseURL/api/v1/setup/bootstrap" -Method Post -Body @{ bootstrap_token = $bootstrapToken; username = 'admin'; email = 'admin@example.com'; password = $adminPassword }
    if ($bootstrap.StatusCode -ne 201) { throw "bootstrap returned $($bootstrap.StatusCode)" }
    $admin = Invoke-Login -BaseURL $baseURL -Identifier 'admin' -Password $adminPassword
    $creator = New-ReadyMember -BaseURL $baseURL -Username 'creator' -Admin $admin
    $other = New-ReadyMember -BaseURL $baseURL -Username 'other' -Admin $admin

    $projectResponse = Invoke-JSON -Uri "$baseURL/api/v1/projects" -Method Post -Body @{ name = 'Site Alpha'; description_markdown = '**Project** <script>bad()</script>' } -Session $admin.Session -CSRFToken $admin.Response.csrf_token
    if ($projectResponse.StatusCode -ne 201) { throw "project create returned $($projectResponse.StatusCode)" }
    $project = ($projectResponse.Content | ConvertFrom-Json).project
    $projectID = $project.id
    foreach ($memberID in @($creator.ID, $other.ID)) {
        $membership = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/members/$memberID" -Method Put -Body $null -Session $admin.Session -CSRFToken $admin.Response.csrf_token
        if ($membership.StatusCode -ne 201) { throw "membership add returned $($membership.StatusCode)" }
    }

    $createTask = Invoke-JSON -Uri "$baseURL/api/v2/projects/$projectID/tasks" -Method Post -Body @{ name = 'Foundation'; goals_markdown = '**Safe goal** [bad](javascript:alert(1))'; description_markdown = '<script>alert(1)</script> useful'; responsible_user_ids = @($creator.ID, $other.ID); target_date = '2026-08-01' } -Session $creator.Session -CSRFToken $creator.Response.csrf_token
    if ($createTask.StatusCode -ne 201) { throw "task create returned $($createTask.StatusCode)" }
    $task = ($createTask.Content | ConvertFrom-Json).task
    if ($task.goals_html -match '(?i)javascript:' -or $task.description_html -match '(?i)<script') { throw 'unsafe Markdown survived sanitization' }
    if ($task.created_by.user_id -ne $creator.ID -or $task.responsible_members.Count -ne 2) { throw 'task ownership or plural responsibility projection was incorrect' }

    $otherRead = Invoke-WebRequest -Uri "$baseURL/api/v2/projects/$projectID/tasks/$($task.id)" -WebSession $other.Session -SkipHttpErrorCheck
    if ($otherRead.StatusCode -ne 200) { throw "responsible Member read returned $($otherRead.StatusCode)" }
    $otherEdit = Invoke-JSON -Uri "$baseURL/api/v2/projects/$projectID/tasks/$($task.id)" -Method Patch -Body @{ name = 'Hijacked'; goals_markdown = ''; description_markdown = ''; responsible_user_ids = @(); target_date = $null; expected_version = 1 } -Session $other.Session -CSRFToken $other.Response.csrf_token
    if ($otherEdit.StatusCode -ne 404) { throw "non-owner edit returned $($otherEdit.StatusCode)" }

    $legacyRead = Invoke-WebRequest -Uri "$baseURL/api/v1/projects/$projectID/tasks/$($task.id)" -WebSession $creator.Session -SkipHttpErrorCheck
    if ($legacyRead.StatusCode -ne 200 -or $null -eq ($legacyRead.Content | ConvertFrom-Json).task.responsible_member) { throw 'V1 compatibility projection failed' }
    $legacyEdit = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/tasks/$($task.id)" -Method Patch -Body @{ name = 'Unsafe flatten'; goals_markdown = ''; description_markdown = ''; responsible_user_id = $other.ID; target_date = $null; expected_version = 1 } -Session $creator.Session -CSRFToken $creator.Response.csrf_token
    if ($legacyEdit.StatusCode -ne 409 -or ($legacyEdit.Content | ConvertFrom-Json).error.code -ne 'task_v2_required') { throw "V1 multiple-assignment edit returned $($legacyEdit.StatusCode)" }

    $creatorEdit = Invoke-JSON -Uri "$baseURL/api/v2/projects/$projectID/tasks/$($task.id)" -Method Patch -Body @{ name = 'Foundation updated'; goals_markdown = 'Updated'; description_markdown = 'Updated'; responsible_user_ids = @($other.ID); target_date = $null; expected_version = 1 } -Session $creator.Session -CSRFToken $creator.Response.csrf_token
    $updatedTask = ($creatorEdit.Content | ConvertFrom-Json).task
    if ($creatorEdit.StatusCode -ne 200 -or $updatedTask.version -ne 2 -or $updatedTask.responsible_members.Count -ne 1 -or $updatedTask.responsible_members[0].user_id -ne $other.ID) { throw "creator edit returned $($creatorEdit.StatusCode)" }
    $staleEdit = Invoke-JSON -Uri "$baseURL/api/v2/projects/$projectID/tasks/$($task.id)" -Method Patch -Body @{ name = 'Stale'; goals_markdown = ''; description_markdown = ''; responsible_user_ids = @(); target_date = $null; expected_version = 1 } -Session $admin.Session -CSRFToken $admin.Response.csrf_token
    if ($staleEdit.StatusCode -ne 409) { throw "stale task edit returned $($staleEdit.StatusCode)" }

    $removeResponsible = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/members/$($other.ID)" -Method Delete -Body $null -Session $admin.Session -CSRFToken $admin.Response.csrf_token
    $afterRemoval = Invoke-WebRequest -Uri "$baseURL/api/v2/projects/$projectID/tasks/$($task.id)" -WebSession $admin.Session -SkipHttpErrorCheck
    $afterRemovalTask = ($afterRemoval.Content | ConvertFrom-Json).task
    if ($removeResponsible.StatusCode -ne 204 -or $afterRemoval.StatusCode -ne 200 -or $afterRemovalTask.version -ne 3 -or $afterRemovalTask.responsible_members.Count -ne 0) { throw 'membership removal did not version and clear the remaining task assignment' }

    $timeline = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/tasks/$($task.id)/timeline" -Method Get -Body $null -Session $admin.Session
    $timelineBody = $timeline.Content | ConvertFrom-Json
    $createdEvent = @($timelineBody.timeline | Where-Object action -eq 'task.created')[0]
    $updatedEvents = @($timelineBody.timeline | Where-Object action -eq 'task.updated')
    if ($timeline.StatusCode -ne 200 -or $createdEvent.metadata.responsible_user_ids.Count -ne 2 -or $null -eq $createdEvent.metadata.responsible_user_id -or $updatedEvents.Count -ne 2) { throw 'timeline did not preserve plural and singular-compatible task assignment chronology' }
    $membershipRemovalEvent = $updatedEvents[-1]
    if ($membershipRemovalEvent.metadata.before.responsible_user_ids.Count -ne 1 -or $membershipRemovalEvent.metadata.after.responsible_user_ids.Count -ne 0 -or $null -ne $membershipRemovalEvent.metadata.after.responsible_user_id) { throw 'timeline membership-removal snapshot was incorrect' }

    $deactivate = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID" -Method Patch -Body @{ name = $project.name; description_markdown = $project.description_markdown; active = $false; expected_version = $project.version } -Session $admin.Session -CSRFToken $admin.Response.csrf_token
    if ($deactivate.StatusCode -ne 200) { throw "project deactivation returned $($deactivate.StatusCode)" }
    $inactiveCreate = Invoke-JSON -Uri "$baseURL/api/v2/projects/$projectID/tasks" -Method Post -Body @{ name = 'Blocked task'; goals_markdown = ''; description_markdown = ''; responsible_user_ids = @(); target_date = $null } -Session $creator.Session -CSRFToken $creator.Response.csrf_token
    if ($inactiveCreate.StatusCode -ne 409 -or ($inactiveCreate.Content | ConvertFrom-Json).error.code -ne 'project_inactive') { throw "inactive project task create returned $($inactiveCreate.StatusCode)" }

    [pscustomobject]@{ TaskCreateV2 = 201; MultipleResponsibilities = 2; V1CompatibilityRead = 200; V1UnsafeFlattenRejected = 409; SafeMarkdown = $true; ResponsibleRead = 200; NonOwnerEdit = 404; CreatorEditV2 = 200; MembershipRemovalClearsOne = $true; TimelineAssignmentHistory = $true; StaleConflict = 409; InactiveProjectCreate = 409 }
}
finally {
    if (-not $process.HasExited) { Stop-Process -Id $process.Id; $process.WaitForExit() }
}

& psql $databaseURL -X -v ON_ERROR_STOP=1 -At -F ' | ' -c "SELECT (SELECT count(*) FROM public.tasks), (SELECT count(*) FROM public.task_responsibilities), (SELECT count(*) FROM public.audit_events WHERE action IN ('task.created','task.updated','authorization.task_denied'));"
if ($LASTEXITCODE -ne 0) { throw 'task-register persistence inspection failed' }
