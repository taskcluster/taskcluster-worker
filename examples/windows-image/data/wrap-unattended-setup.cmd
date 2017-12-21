:: Batch script for running unattended-setup.ps1 and posting the log as we go
:: This is executed by unattended.xml at first login
C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe -ExecutionPolicy ByPass -File E:\unattended-setup.ps1 | E:\taskcluster-worker.exe qemu-guest-tools post-log -
