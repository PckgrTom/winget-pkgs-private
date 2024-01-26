$suc = Get-Content -Path .\success.txt
Get-Content -Raw .\Pckgr_PrivateRepoList.csv | ConvertFrom-Csv | Select-Object -Property AppID -Unique | ForEach-Object { 
    $appid = $_.AppID

    if ($appid -eq 'AppID') {
        return
    }
    if ($suc -contains "'$appid'") {
        echo "skipping $appid"
        return
    }
    
    $URL = "https://vedantmgoyal.vercel.app/api/winget-pkgs/manifests/$appid"
    echo $URL

    $req = Invoke-RestMethod -Uri $URL -Method Get -ContentType "application/json" -StatusCodeVariable status

    if ($status -ne 200) {
        Write-Output "failed to get manifest for $appid"
        Write-Output "failed to get manifest for $appid" >> .\failed.txt
        return
    }

    $req | ForEach-Object { 
        $i = $_.count - 1 ; 
        $j = $_; 
        0..$i | ForEach-Object { 
            $pathofm = Join-Path $PSScriptRoot -ChildPath $j[$_].FileName;

            New-Item -Path $pathofm -ItemType File -Force; 
            $j[$_].Content | Out-File -FilePath $pathofm -Force; 
        } 
    }

    "'$appid'" >> .\success.txt
    Remove-Variable -Name status
}
