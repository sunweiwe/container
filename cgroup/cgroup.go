//Package cgroup handle container's cgroup
package cgroup

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"

	"github.com/sunweiwe/container/utils"
)

func getCgroups(containerId string) []string {

	return []string{
		"/sys/fs/cgroup/memory/container/" + containerId,
		"/sys/fs/cgroup/cpu/container/" + containerId,
		"/sys/fs/cgroup/pid/container/" + containerId,
	}
}

// 原理？
// 创建 cgroup
// 创建文件夹
func CreateCGroups(containerId string, createCGroupDirs bool) {
	cgroups := getCgroups(containerId)

	if createCGroupDirs {
		utils.DoOrDieWithMessage(utils.CreateDirsIfDontExist(cgroups), "Unable to create cgroup directories")
	}

	for _, cgroupDir := range cgroups {
		// utils.DoOrDieWithMessage(os.WriteFile(cgroupDir+"/notify_on_release", []byte("1"), 0755),
		// "Unable to write to cgroup notification file")

		log.Printf("pid is : %s\n", strconv.Itoa(os.Getpid()))

		err := os.WriteFile(cgroupDir+"/cgroup.procs",
			[]byte(strconv.Itoa(os.Getpid())), 0755)
		if err != nil {
			log.Printf("Unable to write to cgroup procs file err:%v\n", err)
		}
		log.Printf("pid err:%v\n", err)

	}
}

func RemoveCGroups(containerId string) {
	cgroups := getCgroups(containerId)

	for _, cgroup := range cgroups {
		utils.DoOrDieWithMessage(os.Remove(cgroup), "Unable to remove cgroup dir")
	}
}

func ConfigureCGroups(containerId string, memory int, swap int, pids int, cpus float64) {

	if memory > 0 {
		setMemoryLimit(containerId, memory, swap)
	}

	if cpus > 0 {
		setCpuLimit(containerId, cpus)
	}

	if pids > 0 {
		setPidsLimit(containerId, pids)
	}
}

func setMemoryLimit(containerId string, memory int, swap int) {
	memoryFilePath := "/sys/fs/cgroup/memory/container/" + containerId +
		"/memory.limit_in_bytes"
	swapFilePath := "/sys/fs/cgroup/memory/container/" + containerId +
		"/memory.memsw.limit_in_bytes"

	utils.DoOrDieWithMessage(os.WriteFile(memoryFilePath,
		[]byte(strconv.Itoa(memory*1024*1024)), 0644),
		"Unable to write memory limit")

	/*
		memory.memsw.limit_in_bytes contains the total amount of memory the
		control group can consume: this includes both swap and RAM.
		If if memory.limit_in_bytes is specified but memory.memsw.limit_in_bytes
		is left untouched, processes in the control group will continue to
		consume swap space.
	*/
	if swap >= 0 {
		utils.DoOrDieWithMessage(
			os.WriteFile(swapFilePath, []byte(strconv.Itoa((memory*1024*1024)+(swap*1024*1024))), 0644),
			"Unable to write memory limit",
		)
	}
}

func setCpuLimit(containerId string, cpus float64) {
	cfsPeriodPath := "/sys/fs/cgroup/cpu/container/" + containerId +
		"/cpu.cfs_period_us"
	cfsQuotaPath := "/sys/fs/cgroup/cpu/container/" + containerId +
		"/cpu.cfs_quota_us"

	if cpus > float64(runtime.NumCPU()) {
		fmt.Printf("Ignoring attempt to set CPU quota to great than number of available CPUs")
		return
	}
	utils.DoOrDieWithMessage(
		os.WriteFile(cfsPeriodPath, []byte(strconv.Itoa(1000000)), 0644),
		"Unable to write CFS period")

	utils.DoOrDieWithMessage(
		os.WriteFile(cfsQuotaPath, []byte(strconv.Itoa(int(1000000*cpus))), 0644),
		"Unable to write CFS quota")
}

func setPidsLimit(containerId string, pids int) {
	maxProcsPath := "/sys/fs/cgroup/pids/container/" + containerId + "/pids.max"
	utils.DoOrDieWithMessage(os.WriteFile(maxProcsPath, []byte(strconv.Itoa(pids)), 0644), "Unable to write pids limit")
}
