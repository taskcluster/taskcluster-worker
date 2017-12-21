# Script executed by setup.ps1 after taskcluster-worker qemu-guest-tools have
# been installed. When this script completes the computer will shutdown and
# image will be created.
write-host "...install whatever should be in this image"
Start-Sleep -s 5
write-host "done..."
