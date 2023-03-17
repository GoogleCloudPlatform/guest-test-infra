// go:build cit
// go:build cit

package windowscontainers

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const dockerVersion = "19.03"
const dockerVolumesDir = "C:\\ProgramData\\docker\\volumes"
const baseContainerImageRepo = "mcr.microsoft.com/windows/servercore"
const baseContainerImageTag = "ltsc2019"

func _getDockerContainerId(containerName string) (string, error) {
	command := fmt.Sprintf("docker ps | findstr -i %s", containerName)
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		return "", err
	}
	containerId := strings.Fields(output.Stdout)[0]
	return containerId, nil
}

func TestDockerIsInstalled(t *testing.T) {
	utils.WindowsContainersOnly(t)
	command := fmt.Sprintf("docker version")
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Cannot get Docker version: %v", err)
	}
	if !strings.Contains(output.Stdout, dockerVersion) {
		t.Fatalf("Docker version output does not contain '%s'.", dockerVersion)
	}
	utils.FailOnPowershellFail(command, "Cannot get Docker version", t)
	command = fmt.Sprintf("docker info")
	utils.FailOnPowershellFail(command, "Cannot get Docker info", t)
}

func TestDockerAvailable(t *testing.T) {
	utils.WindowsContainersOnly(t)
	command := fmt.Sprintf("(Find-Package -providerName DockerMsftProvider -AllVersions).Version")
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Find-Package for DockerMsftProvider had an error: %v", err)
	}

	if !strings.Contains(output.Stdout, dockerVersion) {
		t.Fatalf("Docker Version %s not available in DockerMsftProvider.", dockerVersion)
	}
}

func TestBaseContainerImagesPresent(t *testing.T) {
	utils.WindowsContainersOnly(t)
	command := fmt.Sprintf("docker image list")
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Cannot get Docker image list: %v", err)
	}
	if !strings.Contains(output.Stdout, baseContainerImageRepo) {
		t.Fatalf("Docker image list does not contain '%s'.", baseContainerImageRepo)
	}
}

func testBaseContainerImagesRun(t *testing.T) {
	utils.WindowsContainersOnly(t)
	command := fmt.Sprintf("docker run %s:%s", baseContainerImageRepo, baseContainerImageTag)
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Docker run command had an error: %v", err)
	}
	if !strings.Contains(output.Stdout, "C:\\>") {
		t.Fatalf("Docker run of %s:%s did not complete as expected", baseContainerImageRepo, baseContainerImageTag)
	}
}

func TestCanBuildNewContainerFromDockerfile(t *testing.T) {
	utils.WindowsContainersOnly(t)
	containerDir := "C:\\containers"
	dockerFile := containerDir + "\\hello_dockerfile"
	greeting := "Hello Container"
	containerName := "mycontainer"
	dockerFileContents := `
	FROM %s:%s
	RUN powershell -command "Set-Content C:\greeting.txt \"%s\"
	CMD powershell -command "Get-Content C:\greeting.txt"
	`
	dockerFileContents = fmt.Sprintf(dockerFileContents, baseContainerImageRepo, baseContainerImageTag, greeting)
	command := fmt.Sprintf("New-Item %s -type directory", containerDir)
	utils.FailOnPowershellFail(command, "Error creating directory '%s'", t)

	command = fmt.Sprintf("New-Item %s; Set-Content %s '%s'", dockerFile, dockerFile, dockerFileContents)
	utils.FailOnPowershellFail(command, "Could not create dockerfile", t)

	command = fmt.Sprintf("docker build -f %s %s --tag %s", dockerFile, containerDir, containerName)
	utils.FailOnPowershellFail(command, "Error building container", t)

	output, err := utils.RunPowershellCmd("docker image list")
	if err != nil {
		t.Fatalf("Docker image list failed: %v", err)
	}
	if !strings.Contains(output.Stdout, containerName) {
		t.Fatalf("Container Name %s not in docker image list output", containerName)
	}

	command = fmt.Sprintf("docker run %s", containerName)
	output, err = utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Error running docker container %s: %v", containerName, err)
	}
	if !strings.Contains(output.Stdout, greeting) {
		t.Fatalf("Container output does not contain greeting '%s'", greeting)
	}
}

func TestRunAndKillBackgroundContainer(t *testing.T) {
	utils.WindowsContainersOnly(t)
	containerName := "bg_container"
	command := fmt.Sprintf("docker run --name %s -di %s:%s cmd.exe", containerName, baseContainerImageRepo, baseContainerImageTag)
	utils.FailOnPowershellFail(command, "Error running container", t)
	containerId, err := _getDockerContainerId(containerName)
	if err != nil {
		t.Fatalf("Error getting container ID: %v", err)
	}

	command = fmt.Sprintf("docker exec %s cmd.exe /c 'dir C:\\'", containerId)
	utils.FailOnPowershellFail(command, "Error running exec on container", t)

	command = fmt.Sprintf("docker kill %s", containerId)
	utils.FailOnPowershellFail(command, "Error running kill on container", t)

	command = fmt.Sprintf("docker rm %s", containerId)
	utils.FailOnPowershellFail(command, "Error running rm on container", t)
}

func testContainerCanMountStorageVolume(t *testing.T) {
	utils.WindowsContainersOnly(t)
	containerName := "mycontainer"
	volumeName := "myvolume"
	volumeMount := fmt.Sprintf("%s:C:\\%s_dir", volumeName, volumeName)
	testFileName := "hello.txt"
	testFileContents := "Hello there"
	testFilePath := fmt.Sprintf("%s\\%s\\_data\\%s", dockerVolumesDir, volumeName, testFileName)
	command := fmt.Sprintf("docker volume create %s", volumeName)
	utils.FailOnPowershellFail(command, "Error creating docker volume", t)

	output, err := utils.RunPowershellCmd("docker volume ls")
	if err != nil {
		t.Fatalf("Error listing docker volumes: %v", err)
	}
	if !strings.Contains(output.Stdout, volumeName) {
		t.Fatalf("Could not find '%s' in volume list", volumeName)
	}

	command = fmt.Sprintf("New-Item %s; Set-Content %s \"%s\"", testFilePath, testFilePath, testFileContents)
	utils.FailOnPowershellFail(command, "Error creating test file", t)

	command = fmt.Sprintf("docker run --name %s -v %s -di %s:%s cmd.exe", containerName, volumeMount, baseContainerImageRepo, baseContainerImageTag)
	utils.FailOnPowershellFail(command, "Error running container", t)

	containerId, err := _getDockerContainerId(containerName)
	if err != nil {
		t.Fatalf("Could not get container ID: %v", err)
	}

	command = fmt.Sprintf("docker exec %s cmd.exe /c 'dir C:\\'", containerId)
	output, err = utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Error running exec on container: %v", err)
	}

	if !strings.Contains(output.Stdout, testFileContents) {
		t.Fatalf("Command Stdout '%s' does not contain '%s'", output.Stdout, testFileContents)
	}
}
