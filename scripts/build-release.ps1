param(
    [Parameter(Mandatory = $true)]
    [string]$Version,
    [string]$OutputDir = "dist"
)

$ErrorActionPreference = "Stop"

if ($Version -notmatch '^v\d+\.\d+\.\d+([-.][0-9A-Za-z.-]+)?$') {
    throw "Version must look like v0.1.0 or v0.1.0-rc.1"
}

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$coreRoot = Resolve-Path (Join-Path $repoRoot "..\mcp-center-core")
$kitRoot = Resolve-Path (Join-Path $repoRoot "..\wails-desktop-kit")

function Assert-CleanGitTree([string]$Path, [string]$Name) {
    $status = git -C $Path status --porcelain
    if ($LASTEXITCODE -ne 0) { throw "git status failed for $Name" }
    if ($status) { throw "$Name working tree is not clean. Commit or stash changes before release build." }
}

function Get-Commit([string]$Path, [string]$Name) {
    $value = (git -C $Path rev-parse --short=12 HEAD).Trim()
    if ($LASTEXITCODE -ne 0 -or -not $value) { throw "Cannot resolve Git commit for $Name" }
    return $value
}

Push-Location $repoRoot
try {
    Assert-CleanGitTree $repoRoot "mcp-center"
    Assert-CleanGitTree $coreRoot "mcp-center-core"
    Assert-CleanGitTree $kitRoot "wails-desktop-kit"

    $commit = Get-Commit $repoRoot "mcp-center"
    $coreCommit = Get-Commit $coreRoot "mcp-center-core"
    $kitCommit = Get-Commit $kitRoot "wails-desktop-kit"

    $buildTime = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")
    $ldflags = "-s -w -X github.com/wanstu/mcp-center/internal/buildinfo.Version=$Version -X github.com/wanstu/mcp-center/internal/buildinfo.Commit=$commit -X github.com/wanstu/mcp-center/internal/buildinfo.BuildTime=$buildTime"

    Write-Host "Testing mcp-center..."
    go test ./... -count=1 -timeout 30s
    if ($LASTEXITCODE -ne 0) { throw "mcp-center go test failed" }

    Write-Host "Testing mcp-center-core..."
    Push-Location $coreRoot
    try {
        go test ./... -count=1 -timeout 30s
        if ($LASTEXITCODE -ne 0) { throw "mcp-center-core go test failed" }
    }
    finally {
        Pop-Location
    }

    $releaseDir = Join-Path $OutputDir $Version
    if (Test-Path $releaseDir) { Remove-Item -Recurse -Force $releaseDir }
    New-Item -ItemType Directory -Force $releaseDir | Out-Null

    $targets = @(
        @{ GOOS = "windows"; GOARCH = "amd64"; Ext = ".exe" },
        @{ GOOS = "windows"; GOARCH = "arm64"; Ext = ".exe" },
        @{ GOOS = "linux";   GOARCH = "amd64"; Ext = "" },
        @{ GOOS = "linux";   GOARCH = "arm64"; Ext = "" },
        @{ GOOS = "darwin";  GOARCH = "amd64"; Ext = "" },
        @{ GOOS = "darwin";  GOARCH = "arm64"; Ext = "" }
    )

    $oldGOOS = $env:GOOS
    $oldGOARCH = $env:GOARCH
    $oldCGO = $env:CGO_ENABLED
    try {
        $env:CGO_ENABLED = "0"
        foreach ($target in $targets) {
            $env:GOOS = $target.GOOS
            $env:GOARCH = $target.GOARCH
            $name = "mcp-center-$Version-$($target.GOOS)-$($target.GOARCH)$($target.Ext)"
            $path = Join-Path $releaseDir $name
            Write-Host "Building $name"
            go build -trimpath -ldflags $ldflags -o $path ./cmd/mcp-center
            if ($LASTEXITCODE -ne 0) { throw "Build failed for $($target.GOOS)/$($target.GOARCH)" }
        }
    }
    finally {
        $env:GOOS = $oldGOOS
        $env:GOARCH = $oldGOARCH
        $env:CGO_ENABLED = $oldCGO
    }

    if ($env:OS -eq "Windows_NT") {
        $smokeArch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
        $smokeBinary = Join-Path $releaseDir "mcp-center-$Version-windows-$smokeArch.exe"
        Write-Host "Running release smoke test with $smokeBinary"
        & (Join-Path $PSScriptRoot "smoke-release.ps1") -BinaryPath $smokeBinary -ExpectedVersion $Version
        if ($LASTEXITCODE -ne 0) { throw "Release smoke test failed" }
    }

    $checksums = Get-ChildItem $releaseDir -File |
        Sort-Object Name |
        ForEach-Object {
            $hash = (Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
            "$hash  $($_.Name)"
        }
    $checksums | Set-Content -Encoding ascii (Join-Path $releaseDir "SHA256SUMS.txt")

    $manifest = [ordered]@{
        version = $Version
        build_time_utc = $buildTime
        go_version = (go version)
        source_commits = [ordered]@{
            mcp_center = $commit
            mcp_center_core = $coreCommit
            wails_desktop_kit = $kitCommit
        }
        artifacts = @(
            Get-ChildItem $releaseDir -File |
            Where-Object Name -ne "release-manifest.json" |
            Sort-Object Name |
            ForEach-Object {
                [ordered]@{
                    name = $_.Name
                    size = $_.Length
                    sha256 = (Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
                }
            }
        )
    }
    $manifest | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 (Join-Path $releaseDir "release-manifest.json")

    Write-Host ""
    Write-Host "Release artifacts:"
    Get-ChildItem $releaseDir -File | Sort-Object Name | Format-Table Name, Length -AutoSize
    Write-Host "Output: $((Resolve-Path $releaseDir).Path)"
}
finally {
    Pop-Location
}
