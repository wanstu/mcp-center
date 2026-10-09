param(
    [Parameter(Mandatory = $true)]
    [string]$Version,
    [string]$OutputDir = "dist",
    [string]$KitRoot = ""
)

$ErrorActionPreference = "Stop"

if ($Version -notmatch '^v\d+\.\d+\.\d+([-.][0-9A-Za-z.-]+)?$') {
    throw "Version must look like v0.1.0 or v0.1.0-rc.1"
}

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$coreRoot = Resolve-Path (Join-Path $repoRoot "..\mcp-center-core")
if ([string]::IsNullOrWhiteSpace($KitRoot)) {
    $KitRoot = Join-Path $repoRoot "..\wails-desktop-kit"
}
$KitRoot = (Resolve-Path $KitRoot).Path

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
    Assert-CleanGitTree $KitRoot "wails-desktop-kit (systemd package helper)"
    $expectedKitTag = "v0.10.2"
    $expectedKitCommit = git -C $KitRoot rev-list -n 1 $expectedKitTag
    if ($LASTEXITCODE -ne 0 -or -not $expectedKitCommit) {
        throw "Missing Wails Desktop Kit packaging tag $expectedKitTag"
    }
    $actualKitCommit = git -C $KitRoot rev-parse HEAD
    if ($LASTEXITCODE -ne 0 -or "$actualKitCommit".Trim() -ne "$expectedKitCommit".Trim()) {
        throw "Wails Desktop Kit packaging must be checked out at $expectedKitTag"
    }

    $commit = Get-Commit $repoRoot "mcp-center"
    $coreCommit = Get-Commit $coreRoot "mcp-center-core"
    $kitPackagingCommit = Get-Commit $KitRoot "wails-desktop-kit packaging"
    $kitModule = go list -m -json github.com/wanstu/wails-desktop-kit | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or -not $kitModule.Version) {
        throw "Cannot resolve wails-desktop-kit module version"
    }

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
    # BSD binary redistributions must include the copyright/license notice.
    Copy-Item -LiteralPath (Join-Path $repoRoot "LICENSE") -Destination (Join-Path $releaseDir "LICENSE")

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

    # Reuse the same product wrapper for RCs, official tags and preview packages.
    # Wails Desktop Kit owns .deb/systemd lifecycle; Core remains untouched.
    $absoluteReleaseDir = (Resolve-Path $releaseDir).Path
    & go run ./scripts/package-linux/main.go -version $Version -kit $KitRoot -out $absoluteReleaseDir
    if ($LASTEXITCODE -ne 0) { throw "MCP Center systemd Debian packaging failed" }
    foreach ($arch in @("amd64", "arm64")) {
        $packageSidecar = Join-Path $absoluteReleaseDir "mcp-center-$Version-linux-$arch.deb.sha256"
        if (Test-Path $packageSidecar) { Remove-Item -Force $packageSidecar }
    }

    if ($env:OS -eq "Windows_NT") {
        $smokeArch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
        $smokeBinary = Join-Path $releaseDir "mcp-center-$Version-windows-$smokeArch.exe"
        Write-Host "Running release smoke test with $smokeBinary"
        & (Join-Path $PSScriptRoot "smoke-release.ps1") -BinaryPath $smokeBinary -ExpectedVersion $Version
        if ($LASTEXITCODE -ne 0) { throw "Release smoke test failed" }
    }

    $sourceBundleName = "mcp-center-$Version-source.zip"
    $sourceBundlePath = [System.IO.Path]::GetFullPath((Join-Path $repoRoot (Join-Path $releaseDir $sourceBundleName)))
    Write-Host "Building self-contained source bundle..."
    & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot "build-source-bundle.ps1") -Version $Version -OutputPath $sourceBundlePath
    if ($LASTEXITCODE -ne 0) { throw "Source bundle build failed" }

    $checksums = Get-ChildItem $releaseDir -File |
        Sort-Object Name |
        ForEach-Object {
            $hash = (Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
            "$hash  $($_.Name)"
        }
    $checksums | Set-Content -Encoding ascii (Join-Path $releaseDir "SHA256SUMS.txt")

    $manifest = [ordered]@{
        version = $Version
        license = "BSD-3-Clause"
        build_time_utc = $buildTime
        go_version = (go version)
        source_commits = [ordered]@{
            mcp_center = $commit
            mcp_center_core = $coreCommit
            wails_desktop_kit_packaging = $kitPackagingCommit
            wails_desktop_kit_packaging_tag = $expectedKitTag
        }
        module_dependencies = [ordered]@{
            wails_desktop_kit = [ordered]@{
                version = $kitModule.Version
                sum = $kitModule.Sum
            }
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
