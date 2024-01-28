$fail = Get-Content -Path $PSScriptRoot\failed.txt
Get-Content -Raw $PSScriptRoot\Pckgr_PrivateRepoList.csv | ConvertFrom-Csv | Select-Object -Property AppID -Unique | ForEach-Object { 
    $appid = $_.AppID

    if ($appid -eq 'AppID') {
        return
    }

    $path = Join-Path "$PSScriptRoot" '..' 'manifests' $appid.ToLower()[0] $appid.Replace('.', '/')
    if (Test-Path -Path $path) {
        return
    }

    if ($fail -contains "failed to get manifest for $appid") {
        Write-Output "skipping $appid - failed"
        return
    }
    
    $URL = "https://vedantmgoyal.vercel.app/api/winget-pkgs/manifests/$appid"
    Write-Output $URL

    $req = Invoke-RestMethod -Uri $URL -Method Get -ContentType 'application/json' -StatusCodeVariable status

    if ($status -ne 200) {
        Write-Output "failed to get manifest for $appid"
        Write-Output "failed to get manifest for $appid" >> $PSScriptRoot\failed.txt
        return
    }

    $req | ForEach-Object { 
        $i = $_.count - 1 ; 
        $j = $_; 
        0..$i | ForEach-Object { 
            $pathofm = Join-Path $PSScriptRoot '..' $j[$_].FileName;

            New-Item -Path $pathofm -ItemType File -Force; 
            $j[$_].Content | Out-File -FilePath $pathofm -Force; 
        } 
    }

    Remove-Variable -Name status
}
