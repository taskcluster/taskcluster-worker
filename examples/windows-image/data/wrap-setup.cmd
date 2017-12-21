:: Batch script for running setup.ps1 and posting the log as we go
:: It is executed by setup.lnk at login, created by unattended-setup.ps1
C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe -ExecutionPolicy ByPass -File E:\setup.ps1 | E:\taskcluster-worker.exe qemu-guest-tools post-log -
