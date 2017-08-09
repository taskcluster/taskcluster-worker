# This script is called by wrap-unattended-setup.cmd and it's output is sent
# the taskcluster-worker qemu-build process over the meta-data service.
#
# This script installs wrap-setup.cmd to be executed on login.
# Then wrap-setup.cmd will run setup.ps1, which will remove this script and
# and install taskcluster-worker qemu-guest-tools

write-host "Installing startup link to run wrap-setup.cmd on login"

$Media = "E:\"
$AppDataFolder = "$env:appdata"

New-Item -ItemType Directory -Force -Path "$AppDataFolder\Microsoft\Windows\Start Menu\Programs\Startup"
$WSHShell = New-Object -comObject WScript.Shell
$Shortcut = $WSHShell.CreateShortcut("$AppDataFolder\Microsoft\Windows\Start Menu\Programs\Startup\setup.lnk")
$Shortcut.TargetPath = "E:\wrap-setup.cmd"
$Shortcut.Save()
