//Package cmd cmd
package cmd

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/sunweiwe/container/container"
	"github.com/sunweiwe/container/exec"
	"github.com/sunweiwe/container/image"
	"github.com/sunweiwe/container/network"
	"github.com/sunweiwe/container/utils"
)

var rootCmd = &cobra.Command{
	Use:              "Container",
	Short:            "Container is a like docker",
	Long:             `Container is a self to learn docker by write it`,
	TraverseChildren: true,
}

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "ps for all containers",
	Run: func(cmd *cobra.Command, args []string) {
		container.PrintRunningContainers()
	},
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "run container",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {

		// Create and setup the container0 network bridge we need
		if isUp, _ := network.IsContainerBridgeUp(); !isUp {
			log.Println("Bringing up the container bridge...")
			if err := network.SetupContainerBridge(); err != nil {
				log.Fatalf("Unable to create container0 bridge: %v", err)
			}
		}
		memory := cmd.Flags().Int("memory", -1, "Max RAM to allow in MB")
		swap := cmd.Flags().Int("swap", -1, "Max swap to allow in MB")
		pids := cmd.Flags().Int("pids", -1, "Number of max processes to allow")
		cpus := cmd.Flags().Float64("cpus", -1, "Number of CPU cores to restrict to")
		if err := cmd.Flags().Parse(os.Args[2:]); err != nil {
			fmt.Println("Error parsing: ", err)
		}

		container.InitContainer(cmd.Flags().Args()[0], *memory, *swap, *pids, *cpus, cmd.Flags().Args()[1:])
	},
}

var netnsCmd = &cobra.Command{
	Use:   "setup-netns",
	Short: "create network namespace",
	Run: func(cmd *cobra.Command, args []string) {
		network.SetupNewNetworkNamespace(args[0])
	},
}

var vethCmd = &cobra.Command{
	Use:   "setup-veth",
	Short: "create network namespace",
	Run: func(cmd *cobra.Command, args []string) {
		network.SetupContainerNetWorkInterface(args[0])
	},
}

var childCmd = &cobra.Command{
	Use:   "childe-mode",
	Short: "new shell childe mode",
	Run: func(cmd *cobra.Command, args []string) {
		fs := pflag.FlagSet{}
		fs.ParseErrorsWhitelist.UnknownFlags = true
		memory := fs.Int("memory", -1, "Max RAM to allow in MB")
		swap := fs.Int("swap", -1, "Max swap to allow in MB")
		pids := fs.Int("pids", -1, "Number of max processes to allow")
		cpus := fs.Float64("cpus", -1, "Number of CPU cores to restrict to")

		image := cmd.Flags().Lookup("image").Value.String()

		container.ExecContainerCommand(*memory, *swap, *pids, *cpus, args[0], image, args[1:])
	},
}

var imagesCmd = &cobra.Command{
	Use:   "images",
	Short: "print available image",
	Run: func(cmd *cobra.Command, args []string) {
		image.PrintAvailableImages()
	},
}

var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "exec to running container",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		exec.ExecContainer(args[0])
	},
}

var rmiCmd = &cobra.Command{
	Use:   "rmi",
	Short: "remove the image",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		container.RemoveImageByHash(args[0])
	},
}

func init() {
	childCmd.PersistentFlags().String("image", "", "Container image")
}

func Execute() {
	rand.Seed(time.Now().UnixNano())

	/* We chroot and write to privileged directories. We need to be root */
	if os.Getuid() != 0 {
		log.Fatal("You need root privileges to run this program.")
	}

	/* Create the directories we require */
	if err := utils.InitContainerDirs(); err != nil {
		log.Fatalf("Unable to create requisite directories: %v", err)
	}

	rootCmd.AddCommand(runCmd, netnsCmd, vethCmd, childCmd, psCmd, imagesCmd, execCmd, rmiCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
