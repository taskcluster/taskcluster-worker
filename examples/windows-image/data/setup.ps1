# This script is executed by wrap-setup.cmd which is triggered by setup.lnk
# on the user-login. This script will do the following:
#  1) Remove setup.lnk,
#  2) Install taskcluster-worker qemu-guest-tools,
#  3) Run install.ps1, and,
#  4) Shutdown the machine.

write-host "Removing setup.lnk that triggered this script"
$AppDataFolder = "$env:appdata"
Remove-Item "$AppDataFolder\Microsoft\Windows\Start Menu\Programs\Startup\setup.lnk"

write-host "Installing taskcluster-worker qemu-guest-tools"
$Media = "E:"
$AppDataFolder = "$env:appdata"
$ProgramFilesFolder = "$env:ProgramFiles"
$taskclusterWorkerPath = "$ProgramFilesFolder\taskcluster"

New-Item -ItemType Directory -Force -Path "$taskclusterWorkerPath"
Copy-Item "$Media\taskcluster-worker.exe" "$taskclusterWorkerPath"

$WSHShell = New-Object -comObject WScript.Shell
$Shortcut = $WSHShell.CreateShortcut("$AppDataFolder\Microsoft\Windows\Start Menu\Programs\Startup\taskcluster-worker qemu-guest-tools.lnk")
$Shortcut.TargetPath = "$taskclusterWorkerPath\taskcluster-worker.exe"
$Shortcut.Arguments = "qemu-guest-tools"
$Shortcut.Save()

write-host "Running install.ps1"
& "$Media\install.ps1"

write-host "Setup Completed"
Stop-Computer
