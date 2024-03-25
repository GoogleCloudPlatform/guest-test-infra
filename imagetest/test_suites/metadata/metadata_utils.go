package metadata

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func guestAgentPackageName() string {
	if utils.IsWindows() {
		return "google-compute-engine-windows"
	}
	return "google-guest-agent"
}

func reinstallGuestAgent(ctx context.Context) error {
	pkg := guestAgentPackageName()
	if utils.IsWindows() {
		cmd := exec.CommandContext(ctx, "googet", "install", "-reinstall", pkg)
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
		prep = exec.CommandContext(ctx, "apt", "update", "-y")
		cmd = exec.CommandContext(ctx, "apt", "reinstall", "-y", pkg)
		fallback = exec.CommandContext(ctx, "apt", "install", "-y", "--reinstall", pkg)
	case utils.CheckLinuxCmdExists("dnf"):
		repoArg := "--repo=google-compute-engine"
		cmdTokens := []string{"dnf", "-y", "reinstall", pkg, repoArg}
		cmd = exec.CommandContext(ctx, cmdTokens[0], cmdTokens[1:]...)

		cmdTokens = []string{"dnf", "-y", "upgrade", pkg, repoArg}
		fallback = exec.CommandContext(ctx, cmdTokens[0], cmdTokens[1:]...)
	case utils.CheckLinuxCmdExists("yum"):
		repoArgs := []string{"--disablerepo='*'", "--enablerepo=google-compute-engine"}
		cmdTokens := []string{"yum", "-y", "reinstall", pkg}
		cmdTokens = append(cmdTokens, repoArgs...)
		cmd = exec.CommandContext(ctx, cmdTokens[0], cmdTokens[1:]...)

		cmdTokens = []string{"yum", "-y", "upgrade", pkg}
		cmdTokens = append(cmdTokens, repoArgs...)
		fallback = exec.CommandContext(ctx, cmdTokens[0], cmdTokens[1:]...)
	case utils.CheckLinuxCmdExists("zypper"):
		cmd = exec.CommandContext(ctx, "zypper", "--non-interactive", "install", "--force", pkg)
		fallback = exec.CommandContext(ctx, "zypper", "--non-interactive", "install", "--force", pkg)
		fallback.Env = append(fallback.Env, "ZYPP_LOCK_TIMEOUT=5184000") // A negative value is supposed to wait forever but older versions of libzypp are bugged. This will wait for 24 hours.
	default:
		return fmt.Errorf("could not find a package manager to reinstall %s with", pkg)
	}
	if prep != nil {
		if err := prep.Run(); err != nil {
			return fmt.Errorf("could not prep to reinstall %s: %v", pkg, err)
		}
	}

	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		if fallback != nil {
			fallbackOutput, err := fallback.CombinedOutput()
			if err != nil {
				return fmt.Errorf("could not reinstall %s with fallback: %s, output: %s",
					pkg, err, string(fallbackOutput))
			}
		} else {
			return fmt.Errorf("could not reinstall %s: %s, output: %s", pkg, err, string(cmdOutput))
		}
	}

	return nil
}
