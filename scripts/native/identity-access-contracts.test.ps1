[CmdletBinding()]
param(
    [string]$BaseUrl = 'http://127.0.0.1:3001'
)

$ErrorActionPreference = 'Stop'
$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path

& (Join-Path $scriptRoot 'user-contracts.test.ps1') -BaseUrl $BaseUrl
& (Join-Path $scriptRoot 'application-contracts.test.ps1') -BaseUrl $BaseUrl
& (Join-Path $scriptRoot 'team-contracts.test.ps1') -BaseUrl $BaseUrl

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
$username = "i_$suffix"
$credentials = @{ username = $username; password = 'secret1' } | ConvertTo-Json -Compress
Assert-Response (Invoke-FlowAIRequest Post '/api/users/register' $credentials) 201 'SUCCESS' $true
$login = Invoke-FlowAIRequest Post '/api/users/login' $credentials
Assert-Response $login 201 'SUCCESS' $true
$token = [string]$login.Json.data.token

$application = Invoke-FlowAIRequest Post '/api/apps' (@{ name = "Identity $suffix" } | ConvertTo-Json -Compress) $token
Assert-Response $application 201 'SUCCESS' $true
$applicationId = [string]$application.Json.data.id

$createdKey = Invoke-FlowAIRequest Post '/api/api-keys' (@{
    name = 'contract-key'
    applicationId = $applicationId
    scopes = @('app:read', 'workflow:execute')
} | ConvertTo-Json -Compress) $token
Assert-Response $createdKey 201 'SUCCESS' $true
$rawKey = [string]$createdKey.Json.data.key
if ($rawKey -notmatch '^sk-[0-9a-f]{64}$') {
    throw 'Created API key does not contain 256 bits of lowercase hexadecimal material.'
}
$keyId = [string]$createdKey.Json.data.id

$keys = Invoke-FlowAIRequest Get "/api/api-keys?applicationId=$applicationId" $null $token
Assert-Response $keys 200 'SUCCESS' $true
$listedKey = @($keys.Json.data | Where-Object id -eq $keyId)
if ($listedKey.Count -ne 1 -or $listedKey[0].PSObject.Properties.Name -contains 'key') {
    throw 'API key list is missing the key or leaked plaintext.'
}

$disabledKey = Invoke-FlowAIRequest Patch "/api/api-keys/$keyId/toggle" '{"isActive":false}' $token
Assert-Response $disabledKey 200 'SUCCESS' $true
if ([bool]$disabledKey.Json.data.isActive) {
    throw 'API key toggle did not disable the key.'
}
Assert-Response (Invoke-FlowAIRequest Delete "/api/api-keys/$keyId" $null $token) 200 'SUCCESS' $true

$createdShare = Invoke-FlowAIRequest Post "/api/apps/$applicationId/share" $null $token
Assert-Response $createdShare 201 'SUCCESS' $true
$shareLink = [string]$createdShare.Json.data.shareLink
if ($shareLink -notmatch '^share-[0-9a-f]{32}$') {
    throw 'Share link does not contain 128 bits of lowercase hexadecimal material.'
}
$sameShare = Invoke-FlowAIRequest Get "/api/apps/$applicationId/share" $null $token
Assert-Response $sameShare 200 'SUCCESS' $true
if ($sameShare.Json.data.shareLink -ne $shareLink) {
    throw 'Share generation is not idempotent.'
}

Assert-Response (Invoke-FlowAIRequest Patch "/api/apps/$applicationId/share" '{"isPublic":false}' $token) 200 'SUCCESS' $true
Assert-Response (Invoke-FlowAIRequest Get "/api/share/$shareLink") 404 'NOT_FOUND' $false
Assert-Response (Invoke-FlowAIRequest Patch "/api/apps/$applicationId/share" '{"isPublic":true}' $token) 200 'SUCCESS' $true
$public = Invoke-FlowAIRequest Get "/api/share/$shareLink"
Assert-Response $public 200 'SUCCESS' $true
if ($public.Json.data.id -ne $applicationId) {
    throw 'Public share returned the wrong application.'
}

$embed = Invoke-FlowAIRequest Get "/api/apps/$applicationId/embed" $null $token
Assert-Response $embed 200 'SUCCESS' $true
if ($embed.Json.data.scriptCode -ne $embed.Json.data.scriptTag) {
    throw 'Embed scriptCode compatibility alias is missing.'
}

Assert-Response (Invoke-FlowAIRequest Delete "/api/apps/$applicationId/share" $null $token) 200 'SUCCESS' $true
Assert-Response (Invoke-FlowAIRequest Delete "/api/apps/$applicationId" $null $token) 200 'SUCCESS' $true

Write-Host 'Identity/access contracts passed: users, applications, teams, API keys, sharing, envelopes, and RBAC.'
