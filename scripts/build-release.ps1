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
Push-Location $repoRoot
try {
    $status = git status --porcelain
    if ($LASTEXITCODE -ne 0) { throw "git status failed" }
    if ($status) { throw "Working tree is not clean. Commit or stash changes before release build." }

    $commit = (git rev-parse --short=12 HEAD).Trim()
    if ($LASTEXITCODE -ne 0 -or -not $commit) { throw "Cannot resolve Git commit" }

    $buildTime = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")
    $ldflags = "-s -w -X github.com/wanstu/mcp-center/internal/buildinfo.Version=$Version -X github.com/wanstu/mcp-center/internal/buildinfo.Commit=$commit -X github.com/wanstu/mcp-center/internal/buildinfo.BuildTime=$buildTime"

    Write-Host "Testing..."
    go test ./... -count=1 -timeout 30s
    if ($LASTEXITCODE -ne 0) { throw "go test failed" }

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

    $checksums = Get-ChildItem $releaseDir -File |
        Sort-Object Name |
        ForEach-Object {
            $hash = (Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
            "$hash  $($_.Name)"
        }
    $checksums | Set-Content -Encoding ascii (Join-Path $releaseDir "SHA256SUMS.txt")

    $manifest = [ordered]@{
        version = $Version
        commit = $commit
        build_time_utc = $buildTime
        go_version = (go version)
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
