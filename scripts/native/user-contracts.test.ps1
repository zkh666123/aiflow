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
    if ([string]::IsNullOrWhiteSpace([string]$Response.Json.timestamp)) {
        throw 'Response timestamp is missing.'
    }
}

$suffix = [Guid]::NewGuid().ToString('N').Substring(0, 8)
$username = "u_$suffix"
$credentials = @{ username = $username; password = 'secret1' } | ConvertTo-Json -Compress

$register = Invoke-FlowAIRequest Post '/api/users/register' $credentials
Assert-Response $register 201 'SUCCESS' $true
if ($register.Json.data.username -ne $username -or $null -ne $register.Json.data.passwordHash) {
    throw 'Registration payload is incompatible or leaked password data.'
}

$duplicate = Invoke-FlowAIRequest Post '/api/users/register' $credentials
Assert-Response $duplicate 409 'CONFLICT' $false

$login = Invoke-FlowAIRequest Post '/api/users/login' $credentials
Assert-Response $login 201 'SUCCESS' $true
$token = [string]$login.Json.data.token
if ([string]::IsNullOrWhiteSpace($token)) {
    throw 'Login token is missing.'
}

$profile = Invoke-FlowAIRequest Get '/api/users/profile' $null $token
Assert-Response $profile 200 'SUCCESS' $true
if ($profile.Json.data.username -ne $username) {
    throw 'Authenticated profile does not match the login user.'
}

$update = Invoke-FlowAIRequest Patch '/api/users/profile' '{"avatar":null}' $token
Assert-Response $update 200 'SUCCESS' $true
if ($null -ne $update.Json.data.avatar) {
    throw 'Profile avatar was not cleared.'
}

$lockUsername = "l_$suffix"
$lockCredentials = @{ username = $lockUsername; password = 'secret1' } | ConvertTo-Json -Compress
$lockRegister = Invoke-FlowAIRequest Post '/api/users/register' $lockCredentials
Assert-Response $lockRegister 201 'SUCCESS' $true
$badCredentials = @{ username = $lockUsername; password = 'wrong-password' } | ConvertTo-Json -Compress

$lastFailure = $null
1..5 | ForEach-Object {
    $lastFailure = Invoke-FlowAIRequest Post '/api/users/login' $badCredentials
    Assert-Response $lastFailure 401 'UNAUTHORIZED' $false
}
if ([string]$lastFailure.Json.message -notmatch [char]0x9501) {
    throw 'The fifth failed login did not lock the account.'
}

$lockedCorrectPassword = Invoke-FlowAIRequest Post '/api/users/login' $lockCredentials
Assert-Response $lockedCorrectPassword 401 'UNAUTHORIZED' $false
if ([string]$lockedCorrectPassword.Json.message -notmatch [char]0x9501) {
    throw 'A locked account accepted the correct password.'
}

Write-Host 'User contracts passed: register, duplicate, login, JWT profile, update, and Redis lockout.'
