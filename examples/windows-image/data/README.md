Unattended Windows Install
==========================

./taskcluster-worker qemu-run -V playground/windows/cache/windows-setup.tar.zst -- true

## Windows Activation
 1. Set timezone and date-time correct
 2. `cscript slmgr.vbs -skms-domain mozilla.com` now auto-discovery works
 3. `cscript slmgr.vbs -ato` activate windows


<!---  -C "powershell.exe -ExecutionPolicy ByPass -File 'D:\setup.ps1' | D:\taskcluster-worker.exe qemu-guest-tools post-log -" --->

C:\Windows\WindowsPowerShell\v1.0\powershell.exe -File E:\setup.ps1 | E:\taskcluster-worker.exe qemu-guest-tools post-log -

ping markco for windows help...
