param(
  [string]$Version = $env:CERTYN_INSTALL_VERSION,
  [string]$InstallDir = $(if ($env:CERTYN_INSTALL_DIR) { $env:CERTYN_INSTALL_DIR } else { Join-Path $HOME "AppData\Local\Programs\certyn\bin" }),
  [string]$Repo = $(if ($env:CERTYN_INSTALL_REPO) { $env:CERTYN_INSTALL_REPO } else { "YevheniiGera/certyn-cli" }),
  [string]$BaseUrl = $env:CERTYN_INSTALL_BASE_URL,
  [switch]$Latest,
  [switch]$SkipVerify
)

$ErrorActionPreference = "Stop"

function Write-Info {
  param([string]$Message)
  Write-Host $Message
}

function Get-LatestTag {
  param([string]$Repository)
  $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repository/releases/latest"
  if (-not $release.tag_name) {
    throw "Unable to resolve latest release tag for $Repository"
  }
  return [string]$release.tag_name
}

function Get-Arch {
  $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
  switch ($arch) {
    "x64" { return "amd64" }
    "arm64" { return "arm64" }
    default { throw "Unsupported architecture: $arch" }
  }
}

if ([string]::IsNullOrWhiteSpace($BaseUrl)) {
  $BaseUrl = "https://github.com/$Repo/releases/download"
}
$BaseUrl = $BaseUrl.TrimEnd("/")

if ([string]::IsNullOrWhiteSpace($Version)) {
  if (-not $Latest) {
    Write-Warning "No -Version provided; resolving latest release. Use -Version vX.Y.Z for pinned installs."
  }
  $Version = Get-LatestTag -Repository $Repo
}

$os = "windows"
$arch = Get-Arch
$asset = "certyn_${os}_${arch}.tar.gz"
$assetUrl = "$BaseUrl/$Version/$asset"
$checksumUrl = "$assetUrl.sha256"

$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("certyn-install-" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tempDir | Out-Null

try {
  $archivePath = Join-Path $tempDir $asset
  $checksumPath = "$archivePath.sha256"

  Write-Info "Downloading $assetUrl"
  Invoke-WebRequest -Uri $assetUrl -OutFile $archivePath

  if (-not $SkipVerify) {
    Write-Info "Downloading $checksumUrl"
    Invoke-WebRequest -Uri $checksumUrl -OutFile $checksumPath

    $checksumRaw = (Get-Content -Path $checksumPath -Raw).Trim()
    if ([string]::IsNullOrWhiteSpace($checksumRaw)) {
      throw "Checksum file is empty"
    }
    $expected = ($checksumRaw -split '\s+')[0].ToLowerInvariant()
    $actual = (Get-FileHash -Path $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($expected -ne $actual) {
      throw "Checksum mismatch for $asset. Expected $expected, got $actual"
    }
  }

  tar -xzf $archivePath -C $tempDir

  $binaryPath = Join-Path $tempDir "certyn.exe"
  if (-not (Test-Path -Path $binaryPath -PathType Leaf)) {
    $candidate = Get-ChildItem -Path $tempDir -Recurse -Filter "certyn.exe" | Select-Object -First 1
    if (-not $candidate) {
      throw "certyn.exe was not found in the downloaded archive"
    }
    $binaryPath = $candidate.FullName
  }

  New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
  $destination = Join-Path $InstallDir "certyn.exe"
  Copy-Item -Path $binaryPath -Destination $destination -Force
  Write-Info "Installed certyn to $destination"

  $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
  $pathParts = @()
  if (-not [string]::IsNullOrWhiteSpace($userPath)) {
    $pathParts = $userPath -split ';' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
  }

  $normalizedInstallDir = $InstallDir.TrimEnd('\')
  $hasPath = $false
  foreach ($part in $pathParts) {
    if ($part.TrimEnd('\') -ieq $normalizedInstallDir) {
      $hasPath = $true
      break
    }
  }

  if (-not $hasPath) {
    $newUserPath = if ([string]::IsNullOrWhiteSpace($userPath)) { $InstallDir } else { "$userPath;$InstallDir" }
    [Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
    $env:Path = "$env:Path;$InstallDir"
    Write-Info "Added $InstallDir to user PATH (open a new shell to persist in all sessions)."
  }

  try {
    & $destination --version
  } catch {
    Write-Info "Installed. Run `"$destination --help`" to verify."
  }
}
finally {
  if (Test-Path -Path $tempDir) {
    Remove-Item -Path $tempDir -Recurse -Force
  }
}
