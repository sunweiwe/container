package container

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sunweiwe/container/image"
	"github.com/sunweiwe/container/utils"
)

type RunningContainerInfo struct {
	ContainerId string
	Image       string
	Command     string
	Pid         int
}

// CreateContainerId
func CreateContainerId() string {
	randBytes := make([]byte, 6)
	rand.Read(randBytes)
	return fmt.Sprintf("%02x%02x%02x%02x%02x%02x",
		randBytes[0], randBytes[1], randBytes[2],
		randBytes[3], randBytes[4], randBytes[5])
}

func getDistribution(containerId string) (string, error) {
	var lines []string
	file, err := os.Open("/proc/mounts")
	if err != nil {
		fmt.Println("Unable to read /proc/mounts")
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	for _, line := range lines {
		if strings.Contains(line, containerId) {
			parts := strings.Split(line, " ")
			for _, part := range parts {
				if strings.Contains(part, "lowerdir=") {
					options := strings.Split(part, ",")
					for _, option := range options {
						if strings.Contains(option, "lowerdir=") {
							imagesPath := "/var/lib/container/images"
							leaderString := "lowerdir=" + imagesPath + "/"
							trailerString := option[len(leaderString):]
							imageId := trailerString[:12]
							image, tag := image.GetImageAndTagForHash(imageId)
							return fmt.Sprintf("%s:%s", image, tag), nil
						}
					}
				}
			}
		}
	}
	return "", nil
}

func GetRunningContainerInfoForId(containerId string) (RunningContainerInfo, error) {
	container := RunningContainerInfo{}
	var procs []string
	basePath := "/sys/fs/cgroup/cpu/container/"

	file, err := os.Open(basePath + containerId + "/cgroup.procs")
	if err != nil {
		fmt.Println("Unable to read cgroup.procs")
		return container, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		procs = append(procs, scanner.Text())
	}

	if len(procs) > 0 {
		pid, err := strconv.Atoi(procs[len(procs)-1])
		if err != nil {
			fmt.Println("Unable to read PID")
			return container, err
		}
		cmd, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/exe")
		log.Printf("cmd len is: %d\n", len(cmd))

		if err != nil {
			fmt.Println("Unable to resolve path")
			return container, err
		}
		containerMountPath := "/var/run/container/containers/" + containerId + "/fs/mnt"
		realContainerMountPath, err := filepath.EvalSymlinks(containerMountPath)
		if err != nil {
			fmt.Println("Unable to read command link.")
			return container, err
		}

		image, _ := getDistribution(containerId)
		container = RunningContainerInfo{
			ContainerId: containerId,
			Image:       image,
			Command:     cmd[len(realContainerMountPath):],
			Pid:         pid,
		}
	}

	return container, nil
}

/*
	获取所有正在运行的 container ids
	实现逻辑:
		- 容器会创建很多文件夹，在 /sys/fs/cgroup 目录下
		- 例如, for setting cpu limits, container uses /sys/fs/cgroup/cpu/container
	- 在那个文件夹里面是文件夹，每个文件夹都是当前运行的容器
	- 这些文件夹名就是我们创建的容器id。
	- getContainerInfoForId() does more work. It gathers more information about running
		containers. See struct RunningContainerInfo for details.
	- Inside each of those folders is a "cgroup.procs" file that has the list
		of PIDs of processes inside of that container. From the PID, we can
		get the mounted path from which the process was started. From that
		mounted path, we can get the image of the containers since containers
		are mounted via the overlay file system.
*/
func GetRunningContainers() ([]RunningContainerInfo, error) {
	var containers []RunningContainerInfo
	basePath := "/sys/fs/cgroup/cpu/container"

	entries, err := os.ReadDir(basePath)
	if os.IsNotExist(err) {
		log.Printf("Cgroup cpu container is not exist\n")
		return containers, nil
	}
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			log.Printf("Cgroup cpu container entry is: %s\n", entry.Name())
			container, _ := GetRunningContainerInfoForId(entry.Name())
			if container.Pid > 0 {
				log.Printf("Cgroup cpu container entry is: %s\n", entry.Name())
				containers = append(containers, container)
			}
		}
	}
	return containers, err
}

func PrintRunningContainers() {
	containers, err := GetRunningContainers()
	if err != nil {
		os.Exit(1)
	}
	fmt.Println("CONTAINER ID\tIMAGE\t\tCOMMAND")
	for _, container := range containers {
		fmt.Printf("%s\t%s\t%s\n", container.ContainerId, container.Image, container.Command)
	}
}

func GetPidForRunningContainer(containerId string) int {
	containers, err := GetRunningContainers()
	if err != nil {
		log.Fatalf("Unable to get running containers: %v\n", err)
	}

	for _, c := range containers {
		if c.ContainerId == containerId {
			return c.Pid
		}
	}
	return 0
}

func RemoveImageByHash(imageHash string) {
	imageName, imageTag := image.GetImageAndTagForHash(imageHash)
	if len(imageName) == 0 {
		log.Fatalf("No such image")
	}

	containers, err := GetRunningContainers()
	if err != nil {
		log.Fatalf("Unable to get running containers list: %v\n", err)
	}
	for _, container := range containers {
		if container.Image == imageName+":"+imageTag {
			log.Fatalf("Cannot delete image because it is in use by: %s",
				container.ContainerId)
		}
	}

	utils.DoOrDieWithMessage(os.RemoveAll("/var/lib/container/images/"+imageHash),
		"Unable to remove image directory")

	image.RemoveImageMetadata(imageHash)
}
