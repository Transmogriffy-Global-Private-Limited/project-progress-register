[CmdletBinding()]
param([string]$EnvFile = '.env.local', [int]$Port = 18087)

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
if ($LASTEXITCODE -ne 0 -or [int]$userCount -ne 0) { throw "review verification requires a fully migrated database with zero users; found $userCount" }

$runID = New-RandomToken 6
$env:APP_ENV = 'test'; $env:HTTP_ADDR = "127.0.0.1:$Port"; $env:API_DOCS_ENABLED = 'false'
$env:SESSION_CSRF_KEY = [Convert]::ToBase64String([Security.Cryptography.RandomNumberGenerator]::GetBytes(32))
$env:BOOTSTRAP_TOKEN = New-RandomToken 32
$process = Start-Process -FilePath '.\.local\bin\ppr.exe' -ArgumentList 'serve' -PassThru -WindowStyle Hidden -RedirectStandardOutput ".local\review-$runID-stdout.log" -RedirectStandardError ".local\review-$runID-stderr.log"
$baseURL = "http://127.0.0.1:$Port"

try {
    $started = $false
    foreach ($attempt in 1..50) { try { if ((Invoke-WebRequest -Uri "$baseURL/api/v1/health/ready" -SkipHttpErrorCheck -TimeoutSec 1).StatusCode -eq 200) { $started = $true; break } } catch {}; Start-Sleep -Milliseconds 100 }
    if (-not $started) { throw 'review verification server did not become ready' }

    $adminPassword = (New-RandomToken 24) + 'aA1!'
    $bootstrap = Invoke-JSON -Uri "$baseURL/api/v1/setup/bootstrap" -Method Post -Body @{ bootstrap_token = $env:BOOTSTRAP_TOKEN; username = 'admin'; email = 'admin@example.com'; password = $adminPassword }
    if ($bootstrap.StatusCode -ne 201) { throw "bootstrap returned $($bootstrap.StatusCode)" }
    $admin = Invoke-Login -BaseURL $baseURL -Identifier 'admin' -Password $adminPassword
    $createMember = Invoke-JSON -Uri "$baseURL/api/v1/admin/users" -Method Post -Body @{ username = 'member'; email = 'member@example.com'; role = 'member' } -Session $admin.Session -CSRFToken $admin.Response.csrf_token
    if ($createMember.StatusCode -ne 201) { throw "member creation returned $($createMember.StatusCode)" }
    $credential = $createMember.Content | ConvertFrom-Json
    $firstLogin = Invoke-Login -BaseURL $baseURL -Identifier 'member' -Password $credential.temporary_password
    $memberPassword = (New-RandomToken 24) + 'aA1!'
    $replacePassword = Invoke-JSON -Uri "$baseURL/api/v1/auth/password" -Method Post -Body @{ password = $memberPassword } -Session $firstLogin.Session -CSRFToken $firstLogin.Response.csrf_token
    if ($replacePassword.StatusCode -ne 200) { throw "member password replacement returned $($replacePassword.StatusCode)" }
    $member = Invoke-Login -BaseURL $baseURL -Identifier 'member' -Password $memberPassword

    $projectResponse = Invoke-JSON -Uri "$baseURL/api/v1/projects" -Method Post -Body @{ name = 'Review Site'; description_markdown = 'Review workflow' } -Session $admin.Session -CSRFToken $admin.Response.csrf_token
    if ($projectResponse.StatusCode -ne 201) { throw "project creation returned $($projectResponse.StatusCode)" }
    $projectID = ($projectResponse.Content | ConvertFrom-Json).project.id
    $membership = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/members/$($credential.user.id)" -Method Put -Body $null -Session $admin.Session -CSRFToken $admin.Response.csrf_token
    if ($membership.StatusCode -notin @(200, 201)) { throw "membership creation returned $($membership.StatusCode)" }
    $taskResponse = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/tasks" -Method Post -Body @{ name = 'Foundation'; goals_markdown = ''; description_markdown = ''; responsible_user_id = $null; target_date = $null } -Session $member.Session -CSRFToken $member.Response.csrf_token
    if ($taskResponse.StatusCode -ne 201) { throw "task creation returned $($taskResponse.StatusCode)" }
    $task = ($taskResponse.Content | ConvertFrom-Json).task
    $taskID = $task.id
    $taskUpdate = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/tasks/$taskID" -Method Patch -Body @{ name = 'Foundation and curing'; goals_markdown = ''; description_markdown = 'Track curing records'; responsible_user_id = $null; target_date = $null; expected_version = 1 } -Session $member.Session -CSRFToken $member.Response.csrf_token
    if ($taskUpdate.StatusCode -ne 200 -or ($taskUpdate.Content | ConvertFrom-Json).task.version -ne 2) { throw "task timeline update returned $($taskUpdate.StatusCode)" }

    $metadata = @{ content_markdown = 'Foundation is ready for review'; location = $null; location_unavailable_reason = 'not_supported'; attachments = @() } | ConvertTo-Json -Depth 8 -Compress
    $progressResponse = Invoke-WebRequest -Uri "$baseURL/api/v1/projects/$projectID/tasks/$taskID/progress-updates" -Method Post -Form @{ metadata = $metadata } -Headers @{ 'X-CSRF-Token' = $member.Response.csrf_token; 'Idempotency-Key' = "review-progress-$runID" } -WebSession $member.Session -SkipHttpErrorCheck
    if ($progressResponse.StatusCode -ne 201) { throw "progress creation returned $($progressResponse.StatusCode) $($progressResponse.Content)" }
    $updateID = ($progressResponse.Content | ConvertFrom-Json).progress_update.id

    $commentResponse = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/tasks/$taskID/progress-updates/$updateID/comments" -Method Post -Body @{ content_markdown = 'Please add **curing records**.' } -Session $member.Session -CSRFToken $member.Response.csrf_token
    if ($commentResponse.StatusCode -ne 201) { throw "comment creation returned $($commentResponse.StatusCode)" }
    $comment = ($commentResponse.Content | ConvertFrom-Json).comment
    if ($comment.content_html -notmatch '<strong>curing records</strong>') { throw 'comment Markdown was not rendered as expected' }

    $memberAcceptance = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/tasks/$taskID/progress-updates/$updateID/comments/$($comment.id)/accept" -Method Post -Body $null -Session $member.Session -CSRFToken $member.Response.csrf_token
    if ($memberAcceptance.StatusCode -ne 403) { throw "Member suggestion acceptance returned $($memberAcceptance.StatusCode) instead of 403" }
    $firstAcceptance = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/tasks/$taskID/progress-updates/$updateID/comments/$($comment.id)/accept" -Method Post -Body $null -Session $admin.Session -CSRFToken $admin.Response.csrf_token
    if ($firstAcceptance.StatusCode -ne 201 -or -not ($firstAcceptance.Content | ConvertFrom-Json).created) { throw "first Admin acceptance returned $($firstAcceptance.StatusCode)" }
    $retryAcceptance = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/tasks/$taskID/progress-updates/$updateID/comments/$($comment.id)/accept" -Method Post -Body $null -Session $admin.Session -CSRFToken $admin.Response.csrf_token
    if ($retryAcceptance.StatusCode -ne 200 -or ($retryAcceptance.Content | ConvertFrom-Json).created) { throw "idempotent acceptance retry returned $($retryAcceptance.StatusCode)" }
    $suggestions = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/tasks/$taskID/accepted-suggestions" -Method Get -Body $null -Session $member.Session
    if ($suggestions.StatusCode -ne 200 -or ($suggestions.Content | ConvertFrom-Json).accepted_suggestions.Count -ne 1) { throw "accepted suggestion list returned $($suggestions.StatusCode)" }

    $emptyCurrent = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/tasks/$taskID/assessment" -Method Get -Body $null -Session $member.Session
    if ($emptyCurrent.StatusCode -ne 200 -or $null -ne ($emptyCurrent.Content | ConvertFrom-Json).assessment) { throw 'empty current assessment response was incorrect' }
    $firstAssessment = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/tasks/$taskID/assessment" -Method Put -Body @{ verdict = 'on_track'; remark_markdown = 'Proceed with **superstructure**.'; expected_version = 0 } -Session $admin.Session -CSRFToken $admin.Response.csrf_token
    if ($firstAssessment.StatusCode -ne 201 -or ($firstAssessment.Content | ConvertFrom-Json).assessment.version -ne 1) { throw "first assessment returned $($firstAssessment.StatusCode)" }
    $secondAssessment = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/tasks/$taskID/assessment" -Method Put -Body @{ verdict = 'needs_attention'; remark_markdown = 'Upload curing records.'; expected_version = 1 } -Session $admin.Session -CSRFToken $admin.Response.csrf_token
    if ($secondAssessment.StatusCode -ne 201 -or ($secondAssessment.Content | ConvertFrom-Json).assessment.version -ne 2) { throw "second assessment returned $($secondAssessment.StatusCode)" }
    $staleAssessment = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/tasks/$taskID/assessment" -Method Put -Body @{ verdict = 'blocked'; remark_markdown = 'Stale write.'; expected_version = 1 } -Session $admin.Session -CSRFToken $admin.Response.csrf_token
    if ($staleAssessment.StatusCode -ne 409) { throw "stale assessment returned $($staleAssessment.StatusCode) instead of 409" }
    $memberCurrent = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/tasks/$taskID/assessment" -Method Get -Body $null -Session $member.Session
    if ($memberCurrent.StatusCode -ne 200 -or ($memberCurrent.Content | ConvertFrom-Json).assessment.version -ne 2) { throw "Member current assessment returned $($memberCurrent.StatusCode)" }
    $memberHistory = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/tasks/$taskID/assessments" -Method Get -Body $null -Session $member.Session
    if ($memberHistory.StatusCode -ne 403) { throw "Member assessment history returned $($memberHistory.StatusCode) instead of 403" }
    $adminHistory = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/tasks/$taskID/assessments" -Method Get -Body $null -Session $admin.Session
    $history = ($adminHistory.Content | ConvertFrom-Json).assessments
    if ($adminHistory.StatusCode -ne 200 -or $history.Count -ne 2 -or $history[0].version -ne 2 -or $history[1].version -ne 1) { throw "Admin assessment history returned $($adminHistory.StatusCode)" }

    $dashboard = Invoke-JSON -Uri "$baseURL/api/v1/dashboard" -Method Get -Body $null -Session $member.Session
    $dashboardBody = $dashboard.Content | ConvertFrom-Json
    if ($dashboard.StatusCode -ne 200 -or $dashboardBody.totals.project_count -ne 1 -or $dashboardBody.totals.task_count -ne 1 -or $dashboardBody.totals.progress_update_count -ne 1 -or $dashboardBody.totals.accepted_suggestion_count -ne 1 -or $dashboardBody.totals.current_assessments.needs_attention -ne 1) { throw "dashboard returned $($dashboard.StatusCode)" }
    $timeline = Invoke-JSON -Uri "$baseURL/api/v1/projects/$projectID/tasks/$taskID/timeline" -Method Get -Body $null -Session $member.Session
    $timelineActions = @((($timeline.Content | ConvertFrom-Json).timeline).action)
    foreach ($expectedAction in @('task.created', 'task.updated', 'progress.created', 'comment.created', 'suggestion.accepted', 'assessment.created')) {
        if ($expectedAction -notin $timelineActions) { throw "task timeline omitted $expectedAction" }
    }
    $memberAudit = Invoke-JSON -Uri "$baseURL/api/v1/admin/audit?limit=2" -Method Get -Body $null -Session $member.Session
    if ($memberAudit.StatusCode -ne 403) { throw "Member complete audit returned $($memberAudit.StatusCode) instead of 403" }
    $firstAuditPage = Invoke-JSON -Uri "$baseURL/api/v1/admin/audit?limit=2" -Method Get -Body $null -Session $admin.Session
    $firstAuditBody = $firstAuditPage.Content | ConvertFrom-Json
    if ($firstAuditPage.StatusCode -ne 200 -or $firstAuditBody.audit_events.Count -ne 2 -or [string]::IsNullOrWhiteSpace($firstAuditBody.next_cursor)) { throw "first complete audit page returned $($firstAuditPage.StatusCode)" }
    $escapedCursor = [Uri]::EscapeDataString($firstAuditBody.next_cursor)
    $secondAuditPage = Invoke-JSON -Uri "$baseURL/api/v1/admin/audit?limit=2&cursor=$escapedCursor" -Method Get -Body $null -Session $admin.Session
    if ($secondAuditPage.StatusCode -ne 200 -or ($secondAuditPage.Content | ConvertFrom-Json).audit_events.Count -lt 1) { throw "second complete audit page returned $($secondAuditPage.StatusCode)" }

    [pscustomobject]@{ Comment = 201; MemberAccept = 403; AdminAccept = 201; AcceptRetry = 200; Suggestions = 1; AssessmentVersion = 2; MemberHistory = 403; AdminHistory = 2; DashboardProjects = 1; TimelineEvents = $timelineActions.Count; AuditPagination = 200 }
}
finally {
    if (-not $process.HasExited) { Stop-Process -Id $process.Id; $process.WaitForExit() }
}

& psql $databaseURL -X -v ON_ERROR_STOP=1 -At -F ' | ' -c "SELECT (SELECT count(*) FROM public.task_revisions), (SELECT count(*) FROM public.update_comments), (SELECT count(*) FROM public.accepted_suggestions), (SELECT count(*) FROM public.task_assessments), (SELECT count(*) FROM public.audit_events WHERE action IN ('comment.created','suggestion.accepted','assessment.created','authorization.suggestion_denied','authorization.assessment_denied'));"
if ($LASTEXITCODE -ne 0) { throw 'review persistence inspection failed' }
