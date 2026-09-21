$ErrorActionPreference = "Stop"
$dir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $dir

New-Item -ItemType Directory -Force -Path "$dir\lib" | Out-Null
$libDir = "$dir\lib"

Write-Host "==> 1/8 Building Windows amd64 DLL..."
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "1"
$env:CC = "zig cc -target x86_64-windows-gnu"
go build -buildmode=c-shared -o "$libDir\libbtdby4-windows-amd64.dll" .

Write-Host "==> 2/8 Building Windows arm64 DLL..."
$env:GOOS = "windows"
$env:GOARCH = "arm64"
$env:CGO_ENABLED = "1"
$env:CC = "clang"
go build -buildmode=c-shared -o "$libDir\libbtdby4-windows-arm64.dll" .

Write-Host "==> 3/8 Building Linux amd64 SO (glibc)..."
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "1"
$env:CC = "zig cc -target x86_64-linux-gnu"
go build -buildmode=c-shared -o "$libDir\libbtdby4-linux-amd64.so" .

Write-Host "==> 4/8 Building Linux arm64 SO (glibc)..."
$env:GOOS = "linux"
$env:GOARCH = "arm64"
$env:CGO_ENABLED = "1"
$env:CC = "zig cc -target aarch64-linux-gnu"
go build -buildmode=c-shared -o "$libDir\libbtdby4-linux-arm64.so" .

Write-Host "==> 5/8 Building Linux amd64 SO (musl)..."
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "1"
$env:CC = "zig cc -target x86_64-linux-musl"
go build -buildmode=c-shared -o "$libDir\libbtdby4-linux-amd64-musl.so" .

Write-Host "==> 6/8 Building Linux arm64 SO (musl)..."
$env:GOOS = "linux"
$env:GOARCH = "arm64"
$env:CGO_ENABLED = "1"
$env:CC = "zig cc -target aarch64-linux-musl"
go build -buildmode=c-shared -o "$libDir\libbtdby4-linux-arm64-musl.so" .

$env:PATH = "$dir;" + $env:PATH

Write-Host "==> 7/8 Building macOS arm64 Dylib..."
$env:GOOS = "darwin"
$env:GOARCH = "arm64"
$env:CGO_ENABLED = "1"
$env:CC = "zig cc -target aarch64-macos"
go build -tags "netgo,osusergo" -buildmode=c-archive -o "$dir\tmp_darwin_arm64.a" .
zig cc -target aarch64-macos -dynamiclib "-Wl,-force_load,$dir\tmp_darwin_arm64.a" -o "$libDir\libbtdby4-darwin-arm64.dylib"
Remove-Item -Force "$dir\tmp_darwin_arm64.a", "$dir\tmp_darwin_arm64.h" -ErrorAction SilentlyContinue

Write-Host "==> 8/8 Building macOS amd64 Dylib..."
$env:GOOS = "darwin"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "1"
$env:CC = "zig cc -target x86_64-macos"
go build -tags "netgo,osusergo" -buildmode=c-archive -o "$dir\tmp_darwin_amd64.a" .
zig cc -target x86_64-macos -dynamiclib "-Wl,-force_load,$dir\tmp_darwin_amd64.a" -o "$libDir\libbtdby4-darwin-amd64.dylib"
Remove-Item -Force "$dir\tmp_darwin_amd64.a", "$dir\tmp_darwin_amd64.h" -ErrorAction SilentlyContinue

Get-ChildItem -Path "$libDir\*.h" -ErrorAction SilentlyContinue | Remove-Item -Force

Write-Host "`nAll 8 native binaries compiled successfully in ${libDir}:`n"
Get-ChildItem "$libDir" | Select-Object Name, Length, LastWriteTime | Format-Table -AutoSize
