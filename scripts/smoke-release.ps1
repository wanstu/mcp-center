param(
    [Parameter(Mandatory = $true)]
    [string]$BinaryPath,
    [string]$ExpectedVersion = ""
)

$ErrorActionPreference = "Stop"

function Get-FreePort {
    $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
    $listener.Start()
    try { return $listener.LocalEndpoint.Port }
    finally { $listener.Stop() }
}

function Wait-Tcp([int]$Port, [int]$TimeoutSeconds = 15) {
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    while ([DateTime]::UtcNow -lt $deadline) {
        try {
            $client = [System.Net.Sockets.TcpClient]::new()
            $task = $client.ConnectAsync("127.0.0.1", $Port)
            if ($task.Wait(300) -and $client.Connected) {
                $client.Close()
                return
            }
            $client.Close()
        } catch {}
        Start-Sleep -Milliseconds 150
    }
    throw "Timed out waiting for TCP port $Port"
}

function Invoke-CenterJson {
    param(
        [string]$Method,
        [string]$Uri,
        $Body = $null,
        $Session = $null,
        [switch]$CreateSession
    )
    $args = @{
        Method = $Method
        Uri = $Uri
        ErrorAction = "Stop"
    }
    if ($null -ne $Body) {
        $args.ContentType = "application/json"
        $args.Body = ($Body | ConvertTo-Json -Depth 12 -Compress)
    }
    if ($null -ne $Session) {
        $args.WebSession = $Session
    }
    if ($CreateSession) {
        $args.SessionVariable = "script:CenterSession"
    }
    return Invoke-RestMethod @args
}

function Assert-EchoTool {
    param(
        [string]$BaseUrl,
        [string]$McpId,
        [string]$Message
    )
    $tools = Invoke-CenterJson -Method Get -Uri "$BaseUrl/api/mcps/$McpId/tools" -Session $script:CenterSession
    $toolNames = @($tools.tools | ForEach-Object { $_.name })
    if ($toolNames -notcontains "echo") {
        throw "MCP $McpId did not expose echo tool: $($toolNames -join ', ')"
    }

    $result = Invoke-CenterJson -Method Post -Uri "$BaseUrl/api/mcps/$McpId/call" -Session $script:CenterSession -Body @{
        name = "echo"
        arguments = @{ message = $Message }
    }
    $json = $result | ConvertTo-Json -Depth 12 -Compress
    if ($json -notlike "*echo:$Message*") {
        throw "MCP $McpId echo call returned unexpected result: $json"
    }
}

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$binary = (Resolve-Path $BinaryPath).Path
$tempRoot = Join-Path $repoRoot ("tmp\release-smoke-" + [Guid]::NewGuid().ToString("N"))
$helperPath = Join-Path $tempRoot "mcp-center-smoke-upstream.exe"
$dataDir = Join-Path $tempRoot "data"
$centerPort = Get-FreePort
$httpMcpPort = Get-FreePort
$baseUrl = "http://127.0.0.1:$centerPort"
$httpEndpoint = "http://127.0.0.1:$httpMcpPort/mcp"
$centerProcess = $null
$httpProcess = $null
$stdioMcpId = $null

New-Item -ItemType Directory -Force $tempRoot | Out-Null

try {
    Push-Location $repoRoot
    try {
        Write-Host "Building smoke MCP upstream..."
        go build -trimpath -o $helperPath .\scripts\smoke-upstream.go
        if ($LASTEXITCODE -ne 0) { throw "Failed to build smoke MCP upstream" }
    } finally {
        Pop-Location
    }

    $httpProcess = Start-Process -FilePath $helperPath -ArgumentList @("--transport=http", "--listen=127.0.0.1:$httpMcpPort") -PassThru -WindowStyle Hidden
    Wait-Tcp -Port $httpMcpPort

    $centerProcess = Start-Process -FilePath $binary -ArgumentList @(
        "--listen", "127.0.0.1:$centerPort",
        "--base-url", $baseUrl,
        "--data-dir", $dataDir
    ) -PassThru -WindowStyle Hidden
    Wait-Tcp -Port $centerPort

    $version = Invoke-CenterJson -Method Get -Uri "$baseUrl/api/version"
    if ($ExpectedVersion -and $version.version -ne $ExpectedVersion) {
        throw "Expected version $ExpectedVersion, got $($version.version)"
    }

    $setup = Invoke-CenterJson -Method Get -Uri "$baseUrl/api/setup"
    if (-not $setup.needs_bootstrap) {
        throw "Fresh smoke data directory did not require bootstrap"
    }

    Invoke-CenterJson -Method Post -Uri "$baseUrl/api/setup/register" -CreateSession -Body @{
        username = "smokeadmin"
        password = "smoke-password-12345"
    } | Out-Null

    $httpMcp = Invoke-CenterJson -Method Post -Uri "$baseUrl/api/mcps" -Session $script:CenterSession -Body @{
        name = "smoke-http"
        transport = "streamable-http"
        auth_mode = "none"
        endpoint = $httpEndpoint
    }
    $httpMcpId = $httpMcp.mcp.id
    if (-not $httpMcpId) { throw "HTTP MCP creation did not return an id" }
    Write-Host "Testing HTTP MCP tools/list + tools/call..."
    Assert-EchoTool -BaseUrl $baseUrl -McpId $httpMcpId -Message "http-ok"
    Write-Host "HTTP MCP passed."

    $stdioMcp = Invoke-CenterJson -Method Post -Uri "$baseUrl/api/mcps" -Session $script:CenterSession -Body @{
        name = "smoke-stdio"
        transport = "stdio"
        auth_mode = "none"
        executable = $helperPath
        args = @("--transport=stdio")
    }
    $stdioMcpId = $stdioMcp.mcp.id
    if (-not $stdioMcpId) { throw "stdio MCP creation did not return an id" }
    Write-Host "Testing stdio MCP tools/list + tools/call..."
    Assert-EchoTool -BaseUrl $baseUrl -McpId $stdioMcpId -Message "stdio-ok"
    Write-Host "stdio MCP passed."

    Invoke-CenterJson -Method Post -Uri "$baseUrl/api/runtime/$stdioMcpId/disconnect" -Session $script:CenterSession | Out-Null

    Write-Host "Release smoke test passed:"
    Write-Host "  Version: $($version.version)"
    Write-Host "  HTTP MCP: tools/list + tools/call OK"
    Write-Host "  stdio MCP: tools/list + tools/call OK"
}
finally {
    if ($null -ne $centerProcess -and -not $centerProcess.HasExited) {
        Stop-Process -Id $centerProcess.Id -Force -ErrorAction SilentlyContinue
    }
    if ($null -ne $httpProcess -and -not $httpProcess.HasExited) {
        Stop-Process -Id $httpProcess.Id -Force -ErrorAction SilentlyContinue
    }
    Start-Sleep -Milliseconds 150
    Remove-Item -Recurse -Force $tempRoot -ErrorAction SilentlyContinue
}
