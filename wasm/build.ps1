$ErrorActionPreference = "Stop"
$dir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $dir

Write-Host "==> Building btdby4.wasm (GOOS=js GOARCH=wasm)..."
$env:GOOS = "js"
$env:GOARCH = "wasm"
$env:CGO_ENABLED = "0"

go build -ldflags="-s -w" -o "$dir\btdby4.wasm" .

$goroot = go env GOROOT
$wasmExec = Join-Path $goroot "lib\wasm\wasm_exec.js"
if (Test-Path $wasmExec) {
    Copy-Item $wasmExec "$dir\wasm_exec.js" -Force
    Write-Host "==> Copied wasm_exec.js from Go SDK."
}

$file = Get-Item "$dir\btdby4.wasm"
$mb = [math]::Round($file.Length / 1MB, 2)
Write-Host "`nWebAssembly build complete: $($file.Name) ($mb MB)"
