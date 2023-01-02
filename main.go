package main

import (
	"github.com/sunweiwe/container/cmd"
)

// func usage() {
// 	fmt.Println("Welcome to container!")
// 	fmt.Println("Supported commands:")
// 	fmt.Println("container run [--mem] [--swap] [--pids] [--cpus] <image> <command>")
// 	fmt.Println("container exec <container-id> <command>")
// 	fmt.Println("container images")
// 	fmt.Println("container rmi <image-id>")
// 	fmt.Println("container ps")
// }

func main() {
	cmd.Execute()
}
