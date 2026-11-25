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

$user = Get-LocalUser -Name 'svc_backup' -ErrorAction SilentlyContinue
if ($user) {
    Get-Process -IncludeUserName | Where-Object UserName -eq "$env:COMPUTERNAME\svc_backup" |
        Stop-Process -Force -ErrorAction SilentlyContinue
    Remove-LocalUser -SID $user.SID
}

# Found by path so a re-run still finds it after the account is gone. An SSH
# login can leave the hive loaded with no process left; a reboot releases it.
$userProfile = Get-CimInstance Win32_UserProfile | Where-Object LocalPath -eq 'C:\Users\svc_backup'
if ($userProfile) {
    try {
        $userProfile | Remove-CimInstance
    } catch {
        Write-Warning "svc_backup profile is still loaded ($($_.Exception.Message.Trim())). Reboot and run cleanup.ps1 again to remove it."
        exit 1
    }
}

Write-Host 'Winforge CTF removed.'
