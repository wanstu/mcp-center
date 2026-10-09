param(
    [Parameter(Mandatory = $true)]
    [string]$Version,
    [Parameter(Mandatory = $true)]
    [string]$OutputPath
)

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$coreRoot = (Resolve-Path (Join-Path $repoRoot "..\mcp-center-core")).Path
$tempRoot = Join-Path $repoRoot ("tmp\source-bundle-" + [Guid]::NewGuid().ToString("N"))
$bundleName = "mcp-center-$Version-source"
$bundleRoot = Join-Path $tempRoot $bundleName
$centerDir = Join-Path $bundleRoot "mcp-center"
$coreDir = Join-Path $bundleRoot "mcp-center-core"
$verifyRoot = Join-Path $tempRoot "verify"
$centerTar = Join-Path $tempRoot "center.tar"
$coreTar = Join-Path $tempRoot "core.tar"

function Get-Commit([string]$Path, [string]$Name) {
    $value = (git -C $Path rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0 -or -not $value) {
        throw "Cannot resolve Git commit for $Name"
    }
    return $value
}

function Assert-CleanGitTree([string]$Path, [string]$Name) {
    $status = git -C $Path status --porcelain
    if ($LASTEXITCODE -ne 0) { throw "git status failed for $Name" }
    if ($status) { throw "$Name working tree is not clean. Commit or stash changes before source bundle build." }
}

try {
    Assert-CleanGitTree $repoRoot "mcp-center"
    Assert-CleanGitTree $coreRoot "mcp-center-core"

    $centerCommit = Get-Commit $repoRoot "mcp-center"
    $coreCommit = Get-Commit $coreRoot "mcp-center-core"

    Push-Location $repoRoot
    try {
        $kitModule = go list -m -json github.com/wanstu/wails-desktop-kit | ConvertFrom-Json
        if ($LASTEXITCODE -ne 0 -or -not $kitModule.Version) {
            throw "Cannot resolve wails-desktop-kit module version"
        }
    }
    finally {
        Pop-Location
    }

    New-Item -ItemType Directory -Force $centerDir, $coreDir | Out-Null

    git -C $repoRoot archive --format=tar -o $centerTar HEAD
    if ($LASTEXITCODE -ne 0) { throw "mcp-center git archive failed" }
    git -C $coreRoot archive --format=tar -o $coreTar HEAD
    if ($LASTEXITCODE -ne 0) { throw "mcp-center-core git archive failed" }

    tar -xf $centerTar -C $centerDir
    if ($LASTEXITCODE -ne 0) { throw "mcp-center archive extraction failed" }
    tar -xf $coreTar -C $coreDir
    if ($LASTEXITCODE -ne 0) { throw "mcp-center-core archive extraction failed" }
    if (-not (Test-Path (Join-Path $centerDir "LICENSE")) -or -not (Test-Path (Join-Path $coreDir "LICENSE"))) {
        throw "BSD-3-Clause LICENSE files must exist in both Center and Core source trees"
    }

    Push-Location $centerDir
    try {
        Write-Host "Vendoring source-bundle dependencies..."
        go mod vendor
        if ($LASTEXITCODE -ne 0) { throw "go mod vendor failed" }
    }
    finally {
        Pop-Location
    }

    $sourceManifest = [ordered]@{
        version = $Version
        licenses = [ordered]@{
            mcp_center = "BSD-3-Clause"
            mcp_center_core = "BSD-3-Clause"
            appstream_metadata = "CC0-1.0"
        }
        generated_at_utc = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")
        go_version = (go version)
        source_commits = [ordered]@{
            mcp_center = $centerCommit
            mcp_center_core = $coreCommit
        }
        module_dependencies = [ordered]@{
            wails_desktop_kit = [ordered]@{
                version = $kitModule.Version
            }
        }
        build_modes = @(
            "Sibling source: cd mcp-center; go test ./...; go build ./cmd/mcp-center",
            "Offline vendor: cd mcp-center; go test -mod=vendor ./...; go build -mod=vendor ./cmd/mcp-center"
        )
    }
    $sourceManifest | ConvertTo-Json -Depth 6 | Set-Content -Encoding utf8 (Join-Path $bundleRoot "SOURCE-MANIFEST.json")

    $buildDoc = @(
        "# MCP Center $Version Source Bundle",
        "",
        "This archive is self-contained for source review and offline builds.",
        "",
        "Contents:",
        "",
        "- mcp-center/ - MCP Center source at commit $centerCommit",
        "- mcp-center-core/ - reusable Core source at commit $coreCommit",
        "- mcp-center/LICENSE and mcp-center-core/LICENSE - BSD-3-Clause notices for first-party source",
        "- mcp-center/vendor/ - vendored Go dependencies, including Core and Desktop Kit code used by Center",
        "- SOURCE-MANIFEST.json - exact source/version metadata",
        "",
        "MCP Center and MCP Center Core are licensed BSD-3-Clause.",
        "Vendored third-party dependencies retain their own licenses and notices.",
        "",
        "## Normal build",
        "",
        "    cd mcp-center",
        "    go test ./...",
        "    go build ./cmd/mcp-center",
        "",
        "## Offline / hermetic build",
        "",
        "The vendor tree is verified with the sibling Core directory unavailable and Go module network access disabled.",
        "",
        "    cd mcp-center",
        "    go test -mod=vendor ./...",
        "    go build -mod=vendor ./cmd/mcp-center",
        "",
        "Desktop Kit module: github.com/wanstu/wails-desktop-kit $($kitModule.Version)"
    )
    $buildDoc | Set-Content -Encoding utf8 (Join-Path $bundleRoot "BUILD.md")

    if ([System.IO.Path]::IsPathRooted($OutputPath)) {
        $outputFull = [System.IO.Path]::GetFullPath($OutputPath)
    } else {
        $outputFull = [System.IO.Path]::GetFullPath((Join-Path $repoRoot $OutputPath))
    }
    $outputDir = Split-Path -Parent $outputFull
    New-Item -ItemType Directory -Force $outputDir | Out-Null
    if (Test-Path $outputFull) { Remove-Item -Force $outputFull }

    Compress-Archive -Path $bundleRoot -DestinationPath $outputFull -CompressionLevel Optimal

    Write-Host "Verifying source bundle without sibling Core and without module network..."
    Expand-Archive -Path $outputFull -DestinationPath $verifyRoot
    $verifyBundleRoot = Join-Path $verifyRoot $bundleName
    $verifyCenter = Join-Path $verifyBundleRoot "mcp-center"
    $verifyCore = Join-Path $verifyBundleRoot "mcp-center-core"
    Rename-Item $verifyCore ($verifyCore + ".hidden")

    Push-Location $verifyCenter
    $oldGOFLAGS = $env:GOFLAGS
    $oldGOPROXY = $env:GOPROXY
    try {
        $env:GOFLAGS = "-mod=vendor"
        $env:GOPROXY = "off"
        go test ./... -count=1 -timeout 60s
        if ($LASTEXITCODE -ne 0) { throw "source bundle vendor-only tests failed" }
        go build -trimpath -o (Join-Path $verifyRoot "mcp-center-source-proof.exe") ./cmd/mcp-center
        if ($LASTEXITCODE -ne 0) { throw "source bundle vendor-only build failed" }
    }
    finally {
        $env:GOFLAGS = $oldGOFLAGS
        $env:GOPROXY = $oldGOPROXY
        Pop-Location
    }

    Write-Host "Source bundle verified: $outputFull"
}
finally {
    Remove-Item -Recurse -Force $tempRoot -ErrorAction SilentlyContinue
}
