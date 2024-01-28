#Requires -Version 7.2.2
#Requires -Modules powershell-yaml

$ManifestsDir = Join-Path -Path $PSScriptRoot '..' 'manifests'

#region Functions
function Get-VersionLevelDirs {
    param (
        [Parameter(Mandatory = $true)]
        [string]$Path
    )

    # Get all directories in the current directory
    $AllDirs = Get-ChildItem -Path $Path -Directory

    # Initialize an empty hashtable to store package and version level directories
    $VersionLevelDirs = @{}

    # Loop through each directory
    foreach ($dir in $AllDirs) {
        # Get all directories (versions) present in the directory
        $versionDirs = Get-ChildItem $dir.FullName -Directory

        # If there are subdirectories, call this function recursively
        if ($versionDirs) {
            $VersionLevelDirs += Get-VersionLevelDirs -Path $dir.FullName
        } else {
            # Add the directory and its versions to the VersionLevelDirs hashtable
            $package = $dir.Parent.FullName.Remove(0, $ManifestsDir.Length + 3).Split([IO.Path]::DirectorySeparatorChar) -join '.'
            $version = $dir.FullName
            if ($VersionLevelDirs.ContainsKey($package)) {
                $VersionLevelDirs[$package] += $version
            } else {
                $VersionLevelDirs[$package] = @($version)
            }
        }
    }

    return $VersionLevelDirs
}

function Get-LatestVersion {
    param (
        [Parameter(Mandatory = $true)]
        [string[]]$versions
    )
    $ToNatural = { [regex]::Replace($_, '\d+', { $args[0].Value.PadLeft(20) }) }
    return $versions | Sort-Object $ToNatural | Select-Object -Last 1
}
#endregion

# Call the function with the initial path
$PackagesAndVersions = Get-VersionLevelDirs -Path $ManifestsDir

$FinalResult = @()

foreach ($package in $PackagesAndVersions.Keys) {
    $versions = $PackagesAndVersions[$package]
    $latestVersion = Get-LatestVersion -versions $versions
    $latestVersionName = Split-Path -Path $latestVersion -Leaf
    Write-Host "Latest version of $package is $latestVersionName"

    $data = @{}
    $manifestfiles = Get-ChildItem -Path $latestVersion -Filter '*.yaml' -File
    foreach ($manifestfile in $manifestfiles) {
        $manifest = Get-Content -Path $manifestfile.FullName -Raw | ConvertFrom-Yaml
        switch ($manifest.ManifestType) {
            'installer' { 
                $data['Commands'] = $manifest.Commands | Select-Object -Unique
                $data['PackageFamilyName'] = ($manifest.Installers.PackageFamilyName ?? $manifest.PackageFamilyName) | Select-Object -Unique
                $data['ProductCode'] = ($manifest.Installers.ProductCode ?? $manifest.ProductCode) | Select-Object -Unique

                # if any of them is single string, wrap it in an array
                if ($data['Commands'] -is [string]) {
                    $data['Commands'] = @($data['Commands'])
                }
                if ($data['PackageFamilyName'] -is [string]) {
                    $data['PackageFamilyName'] = @($data['PackageFamilyName'])
                }
                if ($data['ProductCode'] -is [string]) {
                    $data['ProductCode'] = @($data['ProductCode'])
                }
            }
            'defaultLocale' { 
                $data['PackageIdentifier'] = $manifest.PackageIdentifier
                $data['PackageName'] = $manifest.PackageName
                $data['PackageVersion'] = $manifest.PackageVersion
                $data['Publisher'] = $manifest.Publisher
                $data['Moniker'] = $manifest.Moniker
                $data['Tags'] = $manifest.Tags | Select-Object -Unique

                # if any of them is single string, wrap it in an array
                if ($data['Tags'] -is [string]) {
                    $data['Tags'] = @($data['Tags'])
                }
            }
        }
    }

    # add to the final result
    $FinalResult += $data
}

$FinalResult | ConvertTo-Json -Depth 10 | Out-File -FilePath "$PSScriptRoot\data.json" -Encoding utf8
