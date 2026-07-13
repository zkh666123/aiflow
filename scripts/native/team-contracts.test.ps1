[CmdletBinding()]
param(
    [string]$BaseUrl = 'http://127.0.0.1:3001'
)

$ErrorActionPreference = 'Stop'

function Invoke-FlowAIRequest {
    param(
        [Parameter(Mandatory)][string]$Method,
        [Parameter(Mandatory)][string]$Path,
        [string]$Body,
        [string]$Token
    )
    $parameters = @{ UseBasicParsing = $true; Method = $Method; Uri = "$BaseUrl$Path" }
    if (-not [string]::IsNullOrWhiteSpace($Body)) {
        $parameters.ContentType = 'application/json'
        $parameters.Body = $Body
    }
    if (-not [string]::IsNullOrWhiteSpace($Token)) {
        $parameters.Headers = @{ Authorization = "Bearer $Token" }
    }
    try {
        $response = Invoke-WebRequest @parameters
        return [pscustomobject]@{ Status = [int]$response.StatusCode; Json = $response.Content | ConvertFrom-Json }
    }
    catch {
        if ($null -eq $_.Exception.Response -or [string]::IsNullOrWhiteSpace($_.ErrorDetails.Message)) { throw }
        return [pscustomobject]@{ Status = [int]$_.Exception.Response.StatusCode; Json = $_.ErrorDetails.Message | ConvertFrom-Json }
    }
}

function Assert-Response {
    param(
        [Parameter(Mandatory)]$Response,
        [Parameter(Mandatory)][int]$Status,
        [Parameter(Mandatory)][string]$Code,
        [Parameter(Mandatory)][bool]$Success
    )
    if ($Response.Status -ne $Status -or $Response.Json.code -ne $Code -or [bool]$Response.Json.success -ne $Success) {
        throw "Unexpected response: expected HTTP $Status/$Code, got HTTP $($Response.Status)/$($Response.Json.code)"
    }
}

function New-TestUser {
    param([Parameter(Mandatory)][string]$Username)
    $body = @{ username = $Username; password = 'secret1' } | ConvertTo-Json -Compress
    $register = Invoke-FlowAIRequest Post '/api/users/register' $body
    Assert-Response $register 201 'SUCCESS' $true
    $login = Invoke-FlowAIRequest Post '/api/users/login' $body
    Assert-Response $login 201 'SUCCESS' $true
    return [pscustomobject]@{
        Id = [string]$register.Json.data.id
        Token = [string]$login.Json.data.token
    }
}

$suffix = [Guid]::NewGuid().ToString('N').Substring(0, 8)
$owner = New-TestUser "o_$suffix"
$admin = New-TestUser "m_$suffix"
$viewer = New-TestUser "v_$suffix"

$application = Invoke-FlowAIRequest Post '/api/apps' (@{ name = "Team App $suffix" } | ConvertTo-Json -Compress) $owner.Token
Assert-Response $application 201 'SUCCESS' $true
$applicationId = [string]$application.Json.data.id

$team = Invoke-FlowAIRequest Post '/api/teams' (@{ name = "Team $suffix" } | ConvertTo-Json -Compress) $owner.Token
Assert-Response $team 201 'SUCCESS' $true
$teamId = [string]$team.Json.data.id
if (@($team.Json.data.members).Count -ne 1 -or $team.Json.data.members[0].role -ne 'owner') {
    throw 'Team creation did not atomically create the owner membership.'
}

$ownerList = Invoke-FlowAIRequest Get '/api/teams' $null $owner.Token
Assert-Response $ownerList 200 'SUCCESS' $true
$listedTeam = @($ownerList.Json.data | Where-Object id -eq $teamId)
if ($listedTeam.Count -ne 1 -or $listedTeam[0].myRole -ne 'owner' -or [int]$listedTeam[0].memberCount -ne 1) {
    throw 'Owner team list role/count is incompatible.'
}

$adminMember = Invoke-FlowAIRequest Post "/api/teams/$teamId/members" (@{ userId = $admin.Id; role = 'admin' } | ConvertTo-Json -Compress) $owner.Token
Assert-Response $adminMember 201 'SUCCESS' $true
$adminMemberId = [string]$adminMember.Json.data.id
$viewerMember = Invoke-FlowAIRequest Post "/api/teams/$teamId/members" (@{ userId = $viewer.Id; role = 'viewer' } | ConvertTo-Json -Compress) $owner.Token
Assert-Response $viewerMember 201 'SUCCESS' $true
$viewerMemberId = [string]$viewerMember.Json.data.id

$viewerDetail = Invoke-FlowAIRequest Get "/api/teams/$teamId" $null $viewer.Token
Assert-Response $viewerDetail 200 'SUCCESS' $true
if ($viewerDetail.Json.data.myRole -ne 'viewer' -or @($viewerDetail.Json.data.members).Count -ne 3) {
    throw 'Viewer cannot read the complete team detail.'
}

$viewerUpdate = Invoke-FlowAIRequest Patch "/api/teams/$teamId" '{"name":"Denied"}' $viewer.Token
Assert-Response $viewerUpdate 403 'FORBIDDEN' $false
$adminUpdate = Invoke-FlowAIRequest Patch "/api/teams/$teamId" '{"description":"managed by admin"}' $admin.Token
Assert-Response $adminUpdate 200 'SUCCESS' $true

$adminShare = Invoke-FlowAIRequest Post "/api/teams/$teamId/apps" (@{ applicationId = $applicationId; permission = 'can_edit' } | ConvertTo-Json -Compress) $admin.Token
Assert-Response $adminShare 403 'FORBIDDEN' $false
$ownerShare = Invoke-FlowAIRequest Post "/api/teams/$teamId/apps" (@{ applicationId = $applicationId; permission = 'can_edit' } | ConvertTo-Json -Compress) $owner.Token
Assert-Response $ownerShare 201 'SUCCESS' $true
$teamApplicationId = [string]$ownerShare.Json.data.id

$viewerApps = Invoke-FlowAIRequest Get '/api/apps' $null $viewer.Token
Assert-Response $viewerApps 200 'SUCCESS' $true
$sharedApp = @($viewerApps.Json.data | Where-Object id -eq $applicationId)
if ($sharedApp.Count -ne 1 -or $sharedApp[0].accessType -ne 'can_edit') {
    throw 'Team application grant did not reach the application list.'
}
$viewerEdit = Invoke-FlowAIRequest Patch "/api/apps/$applicationId" '{"description":"viewer can edit via grant"}' $viewer.Token
Assert-Response $viewerEdit 200 'SUCCESS' $true
$viewerPublish = Invoke-FlowAIRequest Patch "/api/apps/$applicationId/publish" $null $viewer.Token
Assert-Response $viewerPublish 403 'FORBIDDEN' $false

$fullAccess = Invoke-FlowAIRequest Patch "/api/teams/$teamId/apps/$teamApplicationId" '{"permission":"full_access"}' $admin.Token
Assert-Response $fullAccess 200 'SUCCESS' $true
$viewerArchive = Invoke-FlowAIRequest Patch "/api/apps/$applicationId/archive" $null $viewer.Token
Assert-Response $viewerArchive 200 'SUCCESS' $true
$viewerUnarchive = Invoke-FlowAIRequest Patch "/api/apps/$applicationId/unarchive" $null $viewer.Token
Assert-Response $viewerUnarchive 200 'SUCCESS' $true

$promoted = Invoke-FlowAIRequest Patch "/api/teams/$teamId/members/$viewerMemberId" '{"role":"editor"}' $owner.Token
Assert-Response $promoted 200 'SUCCESS' $true
if ($promoted.Json.data.role -ne 'editor') { throw 'Member role update failed.' }

$removedGrant = Invoke-FlowAIRequest Delete "/api/teams/$teamId/apps/$teamApplicationId" $null $admin.Token
Assert-Response $removedGrant 200 'SUCCESS' $true
$revokedApp = Invoke-FlowAIRequest Get "/api/apps/$applicationId" $null $viewer.Token
Assert-Response $revokedApp 403 'FORBIDDEN' $false

$ownerLeave = Invoke-FlowAIRequest Post "/api/teams/$teamId/leave" $null $owner.Token
Assert-Response $ownerLeave 400 'BAD_REQUEST' $false
$adminLeave = Invoke-FlowAIRequest Post "/api/teams/$teamId/leave" $null $admin.Token
Assert-Response $adminLeave 201 'SUCCESS' $true
$adminRevoked = Invoke-FlowAIRequest Get "/api/teams/$teamId" $null $admin.Token
Assert-Response $adminRevoked 403 'FORBIDDEN' $false

$removedMember = Invoke-FlowAIRequest Delete "/api/teams/$teamId/members/$viewerMemberId" $null $owner.Token
Assert-Response $removedMember 200 'SUCCESS' $true
$viewerRevoked = Invoke-FlowAIRequest Get "/api/teams/$teamId" $null $viewer.Token
Assert-Response $viewerRevoked 403 'FORBIDDEN' $false

$deletedTeam = Invoke-FlowAIRequest Delete "/api/teams/$teamId" $null $owner.Token
Assert-Response $deletedTeam 200 'SUCCESS' $true
$missingTeam = Invoke-FlowAIRequest Get "/api/teams/$teamId" $null $owner.Token
Assert-Response $missingTeam 404 'NOT_FOUND' $false

Write-Host 'Team contracts passed: transaction, roles, members, app grants, revocation, leave, and delete.'
