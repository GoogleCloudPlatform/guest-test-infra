if [[ -f /usr/bin/apt ]]; then
	apt -y update && apt -y install fio
elif [[ -f /bin/dnf ]]; then 
	dnf -y install fio
elif [[ -f /bin/yum ]]; then
	yum -y install fio
elif [[ -f /usr/bin/zypper ]]; then
	zypper --non-interactive install fio
else 
	echo "No package managers found to install fio"
fi

errorcode=$?
if [[ errorcode != 0 ]]; then
	echo "Error running install fio command: code $errorcode"
