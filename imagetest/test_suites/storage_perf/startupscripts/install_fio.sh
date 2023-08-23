if [[ -f /usr/bin/apt ]]; then
	sudo apt -y update && sudo apt -y install fio
fi

if [[ -f /bin/yum ]]; then
	sudo yum -y install fio
fi

if [[ -f /usr/bin/zypper ]]; then
	sudo zypper --non-interactive install fio
fi

if [[ -f /bin/dnf ]]; then
	sudo dnf -y install fio
fi

