package metadata

import (
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func reinstallPackage(pkg string) error {
	if utils.IsWindows() {
		cmd := exec.Command("googet", "install", "-reinstall", pkg)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return err
		}
		if err := cmd.Start(); err != nil {
			return err
		}
		time.Sleep(time.Second)
		// Respond to "Reinstall pkg? (y/N):" prompt
		io.WriteString(stdin, "y\r\n")
		return cmd.Wait()
	}
	var cmd, fallback, prep *exec.Cmd
	switch {
	case utils.CheckLinuxCmdExists("apt"):
		prep = exec.Command("apt", "update", "-y")
		cmd = exec.Command("apt", "reinstall", "-y", pkg)
		fallback = exec.Command("apt", "install", "-y", "--reinstall", pkg)
	case utils.CheckLinuxCmdExists("dnf"):
		cmd = exec.Command("dnf", "-y", "reinstall", pkg)
		fallback = exec.Command("dnf", "-y", "upgrade", pkg)
	case utils.CheckLinuxCmdExists("yum"):
		cmd = exec.Command("yum", "-y", "reinstall", pkg)
		fallback = exec.Command("yum", "-y", "upgrade", pkg)
	case utils.CheckLinuxCmdExists("zypper"):
		cmd = exec.Command("zypper", "--non-interactive", "install", "--force", pkg)
		fallback = exec.Command("zypper", "--non-interactive", "install", "--force", pkg)
	default:
		return fmt.Errorf("could not find a package manager to reinstall %s with", pkg)
	}
	if prep != nil {
		if err := prep.Run(); err != nil {
			return fmt.Errorf("could not prep to reinstall %s: %v", pkg, err)
		}
	}
	if err := cmd.Run(); err != nil {
		if fallback != nil {
			if err := fallback.Run(); err != nil {
				return fmt.Errorf("could not reinstall %s with fallback: %s", pkg, err)
			}
		} else {
			return fmt.Errorf("could not reinstall %s: %s", pkg, err)
		}
	}
	return nil
}
