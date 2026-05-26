$ErrorActionPreference = "Stop"

$Version = if ($args.Length -ge 1) { $args[0] } else { "" }
$Upload = $args -contains "--upload"

if ([string]::IsNullOrWhiteSpace($Version)) {
    Write-Host "用法: .\scripts\release-local.ps1 <tag> [--upload]"
    Write-Host "示例: .\scripts\release-local.ps1 v1.0.1"
    Write-Host "示例: .\scripts\release-local.ps1 v1.0.1 --upload"
    exit 1
}

$RootDir = Resolve-Path (Join-Path $PSScriptRoot "..")
$ReleaseDir = Join-Path $RootDir "release"
New-Item -ItemType Directory -Force -Path $ReleaseDir | Out-Null

if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
    Write-Host "未检测到 wails，请先安装："
    Write-Host "go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0"
    exit 1
}

$Goos = (go env GOOS).Trim()
$Goarch = (go env GOARCH).Trim()

if ($Goos -ne "windows" -or $Goarch -ne "amd64") {
    Write-Host "请在 Windows amd64 环境执行此脚本。当前平台: $Goos/$Goarch"
    exit 1
}

$AssetName = "BestSub-windows-amd64.zip"

Write-Host "开始构建 Windows 桌面包..."
wails build -clean -platform windows/amd64

$AssetPath = Join-Path $ReleaseDir $AssetName
if (Test-Path $AssetPath) {
    Remove-Item $AssetPath -Force
}
Compress-Archive -Path (Join-Path $RootDir "build/bin/BestSub.exe") -DestinationPath $AssetPath -Force

Write-Host "构建完成: $AssetPath"

if (-not $Upload) {
    Write-Host "如需上传到 GitHub Release，请追加 --upload"
    exit 0
}

if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
    Write-Host "未检测到 gh，请先安装 GitHub CLI 后重试上传。"
    exit 1
}

gh release view $Version *> $null
if ($LASTEXITCODE -ne 0) {
    Write-Host "Release $Version 不存在，正在创建..."
    gh release create $Version $AssetPath --title "BestSub $Version" --generate-notes
} else {
    Write-Host "Release $Version 已存在，正在上传/覆盖资产..."
    gh release upload $Version $AssetPath --clobber
}

Write-Host "上传完成: $Version -> $AssetName"
