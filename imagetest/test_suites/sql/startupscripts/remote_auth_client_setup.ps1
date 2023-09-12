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