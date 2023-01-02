//Package exec to container
package exec

import (
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/sunweiwe/container/cgroup"
	"github.com/sunweiwe/container/container"
	"github.com/sunweiwe/container/image"
	"github.com/sunweiwe/container/utils"
	"golang.org/x/sys/unix"
)

func ExecContainer(containerId string) {
	pid := container.GetPidForRunningContainer(containerId)
	if pid == 0 {
		log.Fatalf("No such container!")
	}

	baseNsPath := "/proc/" + strconv.Itoa(pid) + "/ns"
	ipcFd, ipcErr := os.Open(baseNsPath + "/ipc")
	mountFd, mountErr := os.Open(baseNsPath + "/mnt")
	netFd, netErr := os.Open(baseNsPath + "/net")
	pidFd, pidErr := os.Open(baseNsPath + "pid")
	utsFd, utsErr := os.Open(baseNsPath + "uts")

	if ipcErr != nil || mountErr != nil || netErr != nil ||
		pidErr != nil || utsErr != nil {
		log.Fatalf("Unable to open namespace files!")
	}

	unix.Setns(int(ipcFd.Fd()), unix.CLONE_NEWIPC)
	unix.Setns(int(mountFd.Fd()), unix.CLONE_NEWNS)
	unix.Setns(int(netFd.Fd()), unix.CLONE_NEWNET)
	unix.Setns(int(pidFd.Fd()), unix.CLONE_NEWPID)
	unix.Setns(int(utsFd.Fd()), unix.CLONE_NEWUTS)

	containerConfig, err := container.GetRunningContainerInfoForId(containerId)
	if err != nil {
		log.Fatalf("Unable to get container configuration")
	}
	imageNameAndTag := strings.Split(containerConfig.Image, ":")
	exists, imageHash := image.ImageExistByTag(imageNameAndTag[0], imageNameAndTag[1])
	if !exists {
		log.Fatalf("Unable to get image details")
	}

	imageConfig := image.ParseContainerConfig(imageHash)
	containerMountPath := "/var/run/container/containers/" + containerId + "/fs/mnt"
	cgroup.CreateCGroups(containerId, false)
	utils.DoOrDieWithMessage(unix.Chroot(containerMountPath), "Unable to chroot!")
	os.Chdir("/")
	cmd := exec.Command(os.Args[3], os.Args[4:]...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Env = imageConfig.Config.Env
	utils.DoOrDieWithMessage(cmd.Run(), "Unable to exec command in container")
}
