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

    $parameters = @{
        UseBasicParsing = $true
        Method = $Method
        Uri = "$BaseUrl$Path"
    }
    if (-not [string]::IsNullOrWhiteSpace($Body)) {
        $parameters.ContentType = 'application/json'
        $parameters.Body = $Body
    }
    if (-not [string]::IsNullOrWhiteSpace($Token)) {
        $parameters.Headers = @{ Authorization = "Bearer $Token" }
    }

    try {
        $response = Invoke-WebRequest @parameters
        return [pscustomobject]@{
            Status = [int]$response.StatusCode
            Json = $response.Content | ConvertFrom-Json
        }
    }
    catch {
        if ($null -eq $_.Exception.Response -or [string]::IsNullOrWhiteSpace($_.ErrorDetails.Message)) {
            throw
        }
        return [pscustomobject]@{
            Status = [int]$_.Exception.Response.StatusCode
            Json = $_.ErrorDetails.Message | ConvertFrom-Json
        }
    }
}

function Assert-Response {
    param(
        [Parameter(Mandatory)]$Response,
        [Parameter(Mandatory)][int]$Status,
        [Parameter(Mandatory)][string]$Code,
        [Parameter(Mandatory)][bool]$Success
    )

    if ($Response.Status -ne $Status) {
        throw "Expected HTTP $Status, got $($Response.Status)"
    }
    if ($Response.Json.code -ne $Code -or [bool]$Response.Json.success -ne $Success) {
        throw "Unexpected response envelope for HTTP $Status"
    }
}

$suffix = [Guid]::NewGuid().ToString('N').Substring(0, 8)
$username = "a_$suffix"
$credentials = @{ username = $username; password = 'secret1' } | ConvertTo-Json -Compress
Assert-Response (Invoke-FlowAIRequest Post '/api/users/register' $credentials) 201 'SUCCESS' $true
$login = Invoke-FlowAIRequest Post '/api/users/login' $credentials
Assert-Response $login 201 'SUCCESS' $true
$token = [string]$login.Json.data.token

$unauthorized = Invoke-FlowAIRequest Get '/api/apps'
Assert-Response $unauthorized 401 'UNAUTHORIZED' $false

$invalid = Invoke-FlowAIRequest Post '/api/apps' '{"name":"App","ownerId":"other"}' $token
Assert-Response $invalid 400 'BAD_REQUEST' $false

$createBody = @{
    name = "Application $suffix"
    description = 'initial description'
    icon = 'app-icon'
} | ConvertTo-Json -Compress
$created = Invoke-FlowAIRequest Post '/api/apps' $createBody $token
Assert-Response $created 201 'SUCCESS' $true
$applicationId = [string]$created.Json.data.id
if ([string]::IsNullOrWhiteSpace($applicationId) -or $created.Json.data.status -ne 'draft') {
    throw 'Application creation payload is invalid.'
}

$list = Invoke-FlowAIRequest Get '/api/apps' $null $token
Assert-Response $list 200 'SUCCESS' $true
$listed = @($list.Json.data | Where-Object id -eq $applicationId)
if ($listed.Count -ne 1 -or $listed[0].accessType -ne 'owner') {
    throw 'Owned application is missing or has the wrong accessType.'
}

$detail = Invoke-FlowAIRequest Get "/api/apps/$applicationId" $null $token
Assert-Response $detail 200 'SUCCESS' $true
if ([string]$detail.Json.data.userId -ne [string]$login.Json.data.user.id -or @($detail.Json.data.workflows).Count -ne 0) {
    throw 'Application detail owner/workflows fields are incompatible.'
}

$updated = Invoke-FlowAIRequest Patch "/api/apps/$applicationId" '{"name":"Updated App","description":null}' $token
Assert-Response $updated 200 'SUCCESS' $true
if ($updated.Json.data.name -ne 'Updated App' -or $null -ne $updated.Json.data.description) {
    throw 'Application patch did not preserve explicit null semantics.'
}

$transitions = @(
    @{ path = 'publish'; status = 'published' },
    @{ path = 'unpublish'; status = 'draft' },
    @{ path = 'archive'; status = 'archived' },
    @{ path = 'unarchive'; status = 'draft' }
)
foreach ($transition in $transitions) {
    $response = Invoke-FlowAIRequest Patch "/api/apps/$applicationId/$($transition.path)" $null $token
    Assert-Response $response 200 'SUCCESS' $true
    if ($response.Json.data.status -ne $transition.status) {
        throw "Application transition $($transition.path) returned $($response.Json.data.status)."
    }
}

$deleted = Invoke-FlowAIRequest Delete "/api/apps/$applicationId" $null $token
Assert-Response $deleted 200 'SUCCESS' $true
if (-not [bool]$deleted.Json.data.success) {
    throw 'Application delete payload is incompatible.'
}

$missing = Invoke-FlowAIRequest Get "/api/apps/$applicationId" $null $token
Assert-Response $missing 404 'NOT_FOUND' $false

Write-Host 'Application contracts passed: auth, strict input, CRUD, accessType, status transitions, delete, and 404.'
