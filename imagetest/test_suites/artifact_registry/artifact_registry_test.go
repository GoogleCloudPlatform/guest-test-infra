package artifactregistry

import (
	"flag"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	aptFileName          = "/etc/apt/sources.list.d/artifact-registry-private-repo.list"
	aptCreatePrivateRepo = "deb [ trusted=yes ] ar+https://us-central1-apt.pkg.dev/projects/bct-prod-images apt main"
	aptUpdateCmd         = "DEBIAN_FRONTEND=noninteractive apt update"
	aptListCmd           = "apt-cache search dummy-package"
	yumFileName          = "/etc/yum.repos.d/artifact-registry-private-repo.repo"
	yumCreatePrivateRepo = `[artifact-registry-private-repo]
name=Artifact Registry Private Repo
baseurl=https://us-central1-yum.pkg.dev/projects/bct-prod-images/yum
enabled=1
gpgcheck=1
repo_gpgcheck=0
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
       https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
`
	yumUpdateCmd = "yum makecache --disablerepo='*' --enablerepo='artifact-registry-private-repo'"
	yumListCmd   = "yum --disablerepo='*' --enablerepo='artifact-registry-private-repo' list available"
)

var (
	runtest = flag.Bool("runtest", false, "really run the test")
)

func TestMain(m *testing.M) {
	flag.Parse()
	if *runtest {
		os.Exit(m.Run())
	} else {
		os.Exit(0)
	}
}

// TestPrivateRepoDefaultAuth test that artifact registry plugins work correctly.
func TestPrivateRepoDefaultAuth(t *testing.T) {
	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("couldn't get image from metadata")
	}
	if !isSupportedImages(image) {
		t.Skip("not supported image")
	}
	if err := installPackage(image); err != nil {
		t.Fatalf("plugin is not installed, err %v", err)
	}

	var createCmdRaw, fileName string
	var updateCmd, listCmd []string
	switch {
	case strings.Contains(image, "debian"):
		fileName = aptFileName
		createCmdRaw = aptCreatePrivateRepo
		updateCmd = strings.Split(aptUpdateCmd, " ")
		listCmd = strings.Split(aptListCmd, " ")
	default:
		fileName = yumFileName
		createCmdRaw = yumCreatePrivateRepo
		updateCmd = strings.Split(yumUpdateCmd, " ")
		listCmd = strings.Split(yumListCmd, " ")
	}
	if err := os.WriteFile(fileName, []byte(createCmdRaw), 555); err != nil {
		t.Fatalf("fail to write file, err %v", err)
	}

	cmd := exec.Command(updateCmd[0], updateCmd[1:]...)
	if err := cmd.Run(); err != nil {
		t.Fatalf("faile to run update cmd, err %v", err)
	}
	cmd = exec.Command(listCmd[0], listCmd[1:]...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed run list cmd, err %v", err)
	}
	if !strings.Contains(string(out), "dummy-package") {
		t.Fatal("failed to find package in private repo.")
	}
}

func isSupportedImages(image string) bool {
	for _, prefix := range []string{"debian", "centos", "rhel", "almalinux", "rocky-linux"} {
		if strings.Contains(image, prefix) {
			return true
		}
	}
	return false
}

func installPackage(image string) error {
	var installCmd, checkCmd []string
	switch {
	case strings.Contains(image, "debian"):
		pkg := "apt-transport-artifact-registry"
		installCmd = []string{"apt-get", "-y", "-q", "install", pkg}
		checkCmd = []string{"dpkg", "-l", pkg}
	case strings.Contains(image, "centos-7"), strings.Contains(image, "rhel-7"):
		pkg := "yum-plugin-artifact-registry"
		installCmd = []string{"yum", "-y", "-q", "install", pkg}
		checkCmd = []string{"rpm", "-qa", pkg, " "}
	default:
		pkg := "dnf-plugin-artifact-registry"
		installCmd = []string{"yum", "-y", "-q", "install", pkg}
		checkCmd = []string{"rpm", "-qa", pkg, " "}
	}
	cmd := exec.Command(installCmd[0], installCmd[1:]...)
	if err := cmd.Run(); err != nil {
		return err
	}
	cmd = exec.Command(checkCmd[0], checkCmd[1:]...)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
