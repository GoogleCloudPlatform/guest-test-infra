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
	aptCreatePrivateRepo = `cat | sudo tee /etc/apt/sources.list.d/artifact-registry-private-repo.list <<EOF
deb [ trusted=yes ] ar+https://us-central1-apt.pkg.dev/projects/bct-prod-images apt main
EOF
sudo DEBIAN_FRONTEND=noninteractive apt update
`

	yumCreatePrivateRepo = `cat | sudo tee /etc/yum.repos.d/artifact-registry-private-repo.repo <<EOF
[artifact-registry-private-repo]
name=Artifact Registry Private Repo
baseurl=https://us-central1-yum.pkg.dev/projects/bct-prod-images/yum
enabled=1
gpgcheck=1
repo_gpgcheck=0
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOF
sudo yum makecache --disablerepo='*' --enablerepo='artifact-registry-private-repo'`
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
	if err := installPackage(image); err != nil {
		t.Fatalf("plugin is not installed, err %v", err)
	}

	var createCmdRaw string
	var listCmd []string
	switch {
	case strings.Contains(image, "debian"):
		createCmdRaw = aptCreatePrivateRepo
		listCmd = strings.Split("apt-cache search dummy-package", " ")
	default:
		createCmdRaw = yumCreatePrivateRepo
		listCmd = strings.Split("yum --disablerepo='*' --enablerepo='artifact-registry-private-repo' list available", " ")
	}

	if err := os.WriteFile("create.sh", []byte(createCmdRaw), 755); err != nil {
		t.Fatalf("fail to write file, err %v", err)
	}
	cmd := exec.Command("sh", "create.sh")
	if err := cmd.Run(); err != nil {
		t.Fatalf("faile to run cmd, err %v", err)
	}
	cmd = exec.Command(listCmd[0], listCmd[1:]...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed run cmd, err %v", err)
	}
	if !strings.Contains(string(out), "dummy-package") {
		t.Fatal("Failed to find package in private repo.")
	}
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
