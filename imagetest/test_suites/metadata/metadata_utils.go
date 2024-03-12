package metadata

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// yumDnfRunPackageRepoQuery is a common utility between yum and dnf to run the query command
// provided in the cmd argument and parses its output. The output format is the same both for
// yum and dnf (hence the common utility code).
func yumDnfRunPackageRepoQuery(cmd *exec.Cmd) (string, error) {
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run package manager command: %v", err)
	}

	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "From repo") {
			fields := strings.Fields(line)
			if len(fields) != 4 {
				return "", fmt.Errorf("invalid \"From repo\" line, got %d fields, expected 4", len(fields))
			}
			return fields[3], nil
		}
	}

	return "", nil
}

// yumGetPackageRepo queries the yum package database for the repository pkg was installed from.
func yumGetPackageRepo(pkg string) (string, error) {
	return yumDnfRunPackageRepoQuery(exec.Command("yum", "info", "-C", pkg))
}

// dnfGetPackageRepo queries the dnf package database for the repository pkg was installed from.
func dnfGetPackageRepo(pkg string) (string, error) {
	return yumDnfRunPackageRepoQuery(exec.Command("dnf", "info", "-C", "--installed", pkg))
}

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
		repo, err := dnfGetPackageRepo(pkg)
		if err != nil {
			return err
		}

		repoArg := fmt.Sprintf("--enable-repo=%s", repo)
		cmdTokens := []string{"dnf", "-y", "reinstall", pkg}
		if repo != "" {
			cmdTokens = append(cmdTokens, repoArg)
		}
		cmd = exec.Command(cmdTokens[0], cmdTokens[1:]...)

		cmdTokens = []string{"dnf", "-y", "upgrade", pkg}
		if repo != "" {
			cmdTokens = append(cmdTokens, repoArg)
		}
		fallback = exec.Command(cmdTokens[0], cmdTokens[1:]...)
	case utils.CheckLinuxCmdExists("yum"):
		repo, err := yumGetPackageRepo(pkg)
		if err != nil {
			return err
		}

		repoArg := fmt.Sprintf("--enable-repo=%s", repo)
		cmdTokens := []string{"yum", "-y", "reinstall", pkg}
		if repo != "" {
			cmdTokens = append(cmdTokens, repoArg)
		}
		cmd = exec.Command(cmdTokens[0], cmdTokens[1:]...)

		cmdTokens = []string{"yum", "-y", "upgrade", pkg}
		if repo != "" {
			cmdTokens = append(cmdTokens, repoArg)
		}
		fallback = exec.Command(cmdTokens[0], cmdTokens[1:]...)
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
