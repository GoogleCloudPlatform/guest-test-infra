if [[ -f /usr/bin/apt ]]; then
	apt -y update && apt -y install fio
elif [[ -f /bin/dnf ]]; then 
	dnf -y install fio
elif [[ -f /bin/yum ]]; then
	yum -y install fio
elif [[ -f /usr/bin/zypper ]]; then
	sudo zypper --non-interactive install fio
fi
