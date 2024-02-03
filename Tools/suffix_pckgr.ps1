$fail = Get-Content -Path $PSScriptRoot\failed.txt
$pathsep = [System.IO.Path]::DirectorySeparatorChar
Get-Content -Raw $PSScriptRoot\Pckgr_PrivateRepoList.csv | ConvertFrom-Csv | Select-Object -Property AppID -Unique | ForEach-Object {
    $appid = $_.AppID

    if ($appid -eq 'AppID') {
        return
    }

    if ($fail -contains "failed to get manifest for $appid") {
        Write-Output "skipping $appid - failed"
        return
    }

    $existingPckgrPath = Join-Path "$PSScriptRoot" '..' 'manifests' $appid.ToLower()[0] "$($appid)_Pckgr".Replace('.', '/')
    if (Test-Path -Path $existingPckgrPath -PathType Leaf) {
        return
    }

    $existingpath = Join-Path "$PSScriptRoot" '..' 'manifests' $appid.ToLower()[0] "$($appid)_Pckgr".Replace('.', '/')

    # Depth 1 to avoid sub-packages / sub-directories
    Get-ChildItem $existingpath -File -Depth 1 | ForEach-Object {
        $content = Get-Content -Path $_.FullName -Raw
        $yaml = $content.Replace("PackageIdentifier`: $appid", "PackageIdentifier`: $($appid)_Pckgr")

        $newpath = (Split-Path $_.FullName -Parent).Replace($appid.Replace('.', $pathsep), "$($appid)_Pckgr".Replace('.', $pathsep))

        Remove-Item -Path $_.FullName -Force
        New-Item -Path $newpath -ItemType Directory -Force | Out-Null
        $newfile = Join-Path $newpath $_.Name.Replace($appid, "$($appid)_Pckgr")
        $yaml | Out-File -FilePath $newfile -Encoding utf8 -Force
    }
}
