do {
  Get-Date | Out-File C:\shutdown.txt
  Start-Sleep -Seconds 1
} while($true)
