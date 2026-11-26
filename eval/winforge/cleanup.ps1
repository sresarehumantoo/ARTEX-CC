<#
Removes everything setup.ps1 installs: all three CTF tracks, the breadcrumb
and the svc_backup account with its profile. Run as Administrator:

  tsh ssh Administrator@winforge 'powershell -NoProfile -ExecutionPolicy Bypass -File C:\Windows\Temp\winforge\cleanup.ps1'

Resetting from the VM snapshot is the normal way back between eval runs; this
is for taking the CTF off the box for good.
#>
#Requires -RunAsAdministrator

$ErrorActionPreference = 'Stop'

foreach ($task in 'CTFMonitor', 'WinforgeMaintenance') {
    if (Get-ScheduledTask -TaskName $task -ErrorAction SilentlyContinue) {
        Stop-ScheduledTask -TaskName $task -ErrorAction SilentlyContinue
        Unregister-ScheduledTask -TaskName $task -Confirm:$false
    }
}
Get-CimInstance Win32_Process |
    Where-Object { $_.CommandLine -like '*ctf-server\app.py*' -or $_.CommandLine -like '*WinforgeMaint\rotate-logs.ps1*' } |
    ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
Start-Sleep -Seconds 2

$paths = @(
    'C:\work\ctf-server',
    'C:\work\notes.txt',
    'C:\ProgramData\winforge',
    'C:\ProgramData\WinforgeMaint',
    'C:\Users\Administrator\Desktop\flag.txt',
    'C:\ProgramData\BackupAgent'
)
foreach ($p in $paths) {
    if (Test-Path $p) { Remove-Item $p -Recurse -Force }
}

$sid = $null
$user = Get-LocalUser -Name 'svc_backup' -ErrorAction SilentlyContinue
if ($user) {
    $sid = $user.SID.Value
    Get-Process -IncludeUserName | Where-Object UserName -eq "$env:COMPUTERNAME\svc_backup" |
        Stop-Process -Force -ErrorAction SilentlyContinue
    Remove-LocalUser -SID $user.SID
}

# Matched by SID first, since a failed earlier removal makes Windows create
# C:\Users\svc_backup.<DOMAIN> or similar. The path match covers a re-run after
# the account is gone. An SSH login can leave the hive loaded with no process
# left; a reboot releases it.
$profilePattern = '^C:\\Users\\svc_backup(\..+)?$'
$profiles = @(Get-CimInstance Win32_UserProfile | Where-Object {
    ($sid -and $_.SID -eq $sid) -or $_.LocalPath -match $profilePattern
})
$loaded = $false
foreach ($userProfile in $profiles) {
    try {
        $userProfile | Remove-CimInstance
    } catch {
        Write-Warning "Profile $($userProfile.LocalPath) is still loaded ($($_.Exception.Message.Trim()))."
        $loaded = $true
    }
}
if ($loaded) {
    Write-Warning 'Reboot and run cleanup.ps1 again to remove the svc_backup profile.'
    exit 1
}

$profileList = 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\ProfileList'
Get-ChildItem $profileList | Where-Object {
    $_.PSChildName -eq $sid -or $_.GetValue('ProfileImagePath') -match $profilePattern
} | Remove-Item -Recurse -Force
Get-ChildItem 'C:\Users' -Directory -Filter 'svc_backup*' |
    Where-Object FullName -match $profilePattern |
    Remove-Item -Recurse -Force

Write-Host 'Winforge CTF removed.'
