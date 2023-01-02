//Package container create container
package container

import (
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/sunweiwe/container/cgroup"
	"github.com/sunweiwe/container/common"
	"github.com/sunweiwe/container/image"
	"github.com/sunweiwe/container/network"
	"github.com/sunweiwe/container/utils"
	"golang.org/x/sys/unix"
)

// mem 内存
// swap 交换
// pids
// cups
// 初始化
// 1. 创建容器 id
// 2. 下载 image
// 3. 创建容器工作目录
// 4. 挂载文件 overlay 方式
// 5. 创建 veth pair
// 6. 创建 netns
// 7. 挂载 veth
func InitContainer(imageName string, memory int, swap int, pids int, cpus float64, args []string) {
	containerId := CreateContainerId()
	log.Printf("New container ID: %s\n", containerId)

	// imageHash
	imageHash := image.DownloadImageIfRequired(imageName)
	log.Printf("Image to overlay mount: %s\n", imageHash)

	// create container directories
	createContainerDirectories(containerId)
	// 挂载容器文件系统 overlay
	mountOverlayFileSystem(containerId, imageHash)

	// 设置网络 eth
	if err := network.SetUpVirtualEthOnHost(containerId); err != nil {
		log.Fatalf("Unable to setup eth0 on host %v", err)
	}

	// 创建 namespace ，通过ns
	prepareAndExecuteContainer(memory, swap, pids, cpus, containerId, imageHash, args)
	log.Fatalf("Container done.\n")

	unmountNetworkNamespace(containerId)
	unmountContainerFs(containerId)
	cgroup.RemoveCGroups(containerId)
	os.RemoveAll("/var/run/container/containers/" + containerId)
}

func createContainerDirectories(containerId string) {
	containerHome := "/var/run/container/containers/" + containerId + "/fs"
	containerDirs := []string{containerHome, containerHome + "/mnt", containerHome + "/upperdir", containerHome + "/workdir"}
	if err := utils.CreateDirsIfNotExist(containerDirs); err != nil {
		log.Fatalf("Unable to create required directories: %v\n", err)
	}
}

func mountOverlayFileSystem(containerId string, imageHash string) {
	var srcLayers []string
	pathManifest := "/var/lib/container/images/" + imageHash + "/" + imageHash + ".json"
	mani := common.Manifest{}
	utils.ParseManifest(pathManifest, &mani)
	if len(mani) == 0 || len(mani[0].Layers) == 0 {
		log.Fatal("Could not find any layer.")
	}
	if len(mani) > 1 {
		log.Fatal("I don't know how to handle more than one manifest.")
	}

	imageBasePath := "/var/lib/container/images/" + imageHash
	for _, layer := range mani[0].Layers {
		srcLayers = append([]string{imageBasePath + "/" + layer[:12] + "/fs"}, srcLayers...)
	}

	containerFsHome := "/var/run/container/containers/" + containerId + "/fs"
	mntOptions := "lowerdir=" + strings.Join(srcLayers, ":") + ",upperdir=" + containerFsHome + "/upperdir,workdir=" + containerFsHome + "/workdir"
	if err := unix.Mount("none", containerFsHome+"/mnt", "overlay", 0, mntOptions); err != nil {
		log.Fatalf("Mount failed: %v\n", err)
	}
}

func prepareAndExecuteContainer(memory int, swap int, pids int, cpus float64, containerId string, imageHash string, cmdArgs []string) {
	// setup the network namespace
	cmd := &exec.Cmd{
		Path:   "/proc/self/exe",
		Args:   []string{"/proc/self/exe", "setup-netns", containerId},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	cmd.Run()

	// Namespace and setup the virtual interface
	cmd = &exec.Cmd{
		Path:   "/proc/self/exe",
		Args:   []string{"/proc/self/exe", "setup-veth", containerId},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	cmd.Run()
	/*
		From namespaces(7)
			Namespace Flag            Isolates
			--------- --------------- -------
			Cgroup    CLONE_NEWCGROUP Cgroup root directory
			IPC       CLONE_NEWIPC    System V IPC,
																POSIX message queues
			Network   CLONE_NEWNET    Network devices,
																stacks, ports, etc.
			Mount     CLONE_NEWNS     Mount points
			PID       CLONE_NEWPID    Process IDs
			Time      CLONE_NEWTIME   Boot and monotonic
																clocks
			User      CLONE_NEWUSER   User and group IDs
			UTS       CLONE_NEWUTS    Hostname and NIS
		                                 domain name
	*/
	var opts []string
	if memory > 0 {
		opts = append(opts, "--mem="+strconv.Itoa(memory))
	}

	if swap >= 0 {
		opts = append(opts, "--swaps="+strconv.Itoa(swap))
	}

	if pids > 0 {
		opts = append(opts, "--pids="+strconv.Itoa(pids))
	}

	if cpus > 0 {
		opts = append(opts, "--cpu="+strconv.FormatFloat(cpus, 'f', 1, 64))
	}

	opts = append(opts, "--image="+imageHash)
	args := append([]string{containerId}, cmdArgs...)
	args = append(opts, args...)
	args = append([]string{"childe-mode"}, args...)
	cmd = exec.Command("/proc/self/exe", args...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.SysProcAttr = &unix.SysProcAttr{
		Cloneflags: unix.CLONE_NEWPID | unix.CLONE_NEWNS | unix.CLONE_NEWUTS | unix.CLONE_NEWIPC,
	}

	utils.DoOrDie(cmd.Run())
}

func unmountNetworkNamespace(containerId string) {
	netNsPath := "/var/run/container/net-ns" + "/" + containerId
	if err := unix.Unmount(netNsPath, 0); err != nil {
		log.Fatalf("Unable to unmount network namespace: %v at %s \n", err, netNsPath)
	}
}

// unmountContainerFs
func unmountContainerFs(containerId string) {
	mountedPath := "/var/run/container/containers/" + containerId + "/fs/mnt"
	if err := unix.Unmount(mountedPath, 0); err != nil {
		log.Fatalf("Unable to unmount container fs: %v at %s \n", err, mountedPath)
	}
}

func copyNameServerConfig(containerId string) error {
	resolvFilePaths := []string{
		"/var/run/systemd/resolve/resolv.conf",
		"/etc/containerresolv.conf",
		"/etc/resolv.conf",
	}

	for _, resolvFilePath := range resolvFilePaths {
		if _, err := os.Stat(resolvFilePath); os.IsNotExist(err) {
			continue
		} else {
			return utils.CopyFile(resolvFilePath,
				"/var/run/container/containers/"+containerId+"/fs"+"/mnt/etc/resolv.conf")
		}
	}
	return nil
}

func ExecContainerCommand(memory int, swap int, pids int, cpus float64, containerId string, imageHash string, args []string) {
	mountedPath := "/var/run/container/containers/" + containerId + "/fs/mnt"
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	imgConfig := image.ParseContainerConfig(imageHash)
	utils.DoOrDieWithMessage(unix.Sethostname([]byte(containerId)), "Unable to set hostname")
	utils.DoOrDieWithMessage(network.JoinContainerNetworkNamespace(containerId), "Unable to join container network namespace")
	cgroup.CreateCGroups(containerId, true)
	cgroup.ConfigureCGroups(containerId, memory, swap, pids, cpus)
	utils.DoOrDieWithMessage(copyNameServerConfig(containerId), "Unable to copy resolve.conf")

	//! TODO
	utils.DoOrDieWithMessage(unix.Chroot(mountedPath), "Unable to chroot")
	utils.DoOrDieWithMessage(os.Chdir("/"), "Unable to change directory")

	utils.CreateDirsIfDontExist([]string{"/proc", "/sys"})
	utils.DoOrDieWithMessage(unix.Mount("proc", "/proc", "proc", 0, ""), "Unable to mount proc")
	utils.DoOrDieWithMessage(unix.Mount("tmpfs", "/tmp", "tmpfs", 0, ""), "Unable to mount tmpfs")
	utils.DoOrDieWithMessage(unix.Mount("tmpfs", "/dev", "tmpfs", 0, ""), "Unable to mount tmpfs on /dev")

	utils.CreateDirsIfDontExist([]string{"/dev/pts"})
	utils.DoOrDieWithMessage(unix.Mount("devpts", "/dev/pts", "devpts", 0, ""), "Unable to mount devpts")
	utils.DoOrDieWithMessage(unix.Mount("sysfs", "/sys", "sysfs", 0, ""), "Unable to mount sysfs")

	network.SetupLocalInterface()

	cmd.Env = imgConfig.Config.Env
	cmd.Run()
	utils.DoOrDie(unix.Unmount("/dev/pts", 0))
	utils.DoOrDie(unix.Unmount("/dev", 0))
	utils.DoOrDie(unix.Unmount("/sys", 0))
	utils.DoOrDie(unix.Unmount("/proc", 0))
	utils.DoOrDie(unix.Unmount("/tmp", 0))
}
