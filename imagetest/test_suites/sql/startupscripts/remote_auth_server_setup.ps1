$sqlservice = Get-Service 'MSSQLSERVER'
$sqlservice.WaitForStatus('Running', '00:10:00')
Write-Host 'MSSQLSERVER is now running.'

$cn = [ADSI]"WinNT://$env:COMPUTERNAME"
$user = $cn.Create('User', 'SqlTests')
$user.SetPassword('remoting@123')
$user.SetInfo()
$user.description = 'Admin user to install new software'
$user.SetInfo()
$group = [ADSI]"WinNT://$env:COMPUTERNAME/Administrators"
$group.Add($user.Path)

$AUTH_SCRIPT = 'https://storage.googleapis.com/windows-utils/change_auth.sql'
Invoke-WebRequest -Uri $AUTH_SCRIPT -OutFile c:\\change_auth.sql

try {
    sqlcmd -S localhost -i c:\change_auth.sql
} catch {
    Write-Host "Failed to set auth config: $_"
} finally {
    Restart-Service MSSQLSERVER
    Write-Host "auth config updated"
}
