//Package network to build container net
package network

import (
	"fmt"
	"log"
	"math/rand"
	"net"

	"github.com/sunweiwe/container/utils"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const (
	nsMountBase = "/var/run/container/net-ns"
)

func createIPAddress() string {
	byte1 := rand.Intn(254)
	byte2 := rand.Intn(254)
	return fmt.Sprintf("172.29.%d.%d", byte1, byte2)
}

func SetUpVirtualEthOnHost(containerId string) error {
	veth0 := "veth0_" + containerId[:6]
	veth1 := "veth1_" + containerId[:6]

	// veth pair
	LinkAttrs := netlink.NewLinkAttrs()
	LinkAttrs.Name = veth0

	veth0Struct := &netlink.Veth{
		LinkAttrs:        LinkAttrs,
		PeerName:         veth1,
		PeerHardwareAddr: createMACAddress(),
	}

	if err := netlink.LinkAdd(veth0Struct); err != nil {
		return err
	}

	netlink.LinkSetUp(veth0Struct)
	containerBridge, _ := netlink.LinkByName("container0")

	netlink.LinkSetMaster(veth0Struct, containerBridge)
	return nil
}

// 生成固定的地址
func createMACAddress() net.HardwareAddr {
	hw := make(net.HardwareAddr, 6)
	hw[0] = 0x02
	hw[1] = 0x42

	rand.Read(hw[2:])
	return hw
}

//
func SetupNewNetworkNamespace(containerId string) {

	// base url
	_ = utils.CreateDirsIfNotExist([]string{nsMountBase})
	// path
	nsMount := nsMountBase + "/" + containerId

	if _, err := unix.Open(nsMount, unix.O_RDONLY|unix.O_CREAT|unix.O_EXCL, 0644); err != nil {
		log.Fatalf("Unable to open networks bind file: %v\n", err)
	}

	fd, err := unix.Open("/proc/self/ns/net", unix.O_RDONLY, 0)
	defer unix.Close(fd)
	if err != nil {
		log.Fatalf("Unable to open: %v\n", err)
	}

	if err := unix.Unshare(unix.CLONE_NEWNET); err != nil {
		log.Fatalf("Unshare system call failed: %v\n", err)
	}

	//
	if err := unix.Mount("/proc/self/ns/net", nsMount, "bind", unix.MS_BIND, ""); err != nil {
		log.Fatalf("Mount system call failed: %v\n", err)
	}

	//
	if err := unix.Setns(fd, unix.CLONE_NEWNET); err != nil {
		log.Fatalf("Setns system call failed: %v\n", err)
	}

}

func SetupContainerNetWorkInterface(containerId string) {
	nsMount := nsMountBase + "/" + containerId

	fd, err := unix.Open(nsMount, unix.O_RDONLY, 0)
	defer unix.Close(fd)
	if err != nil {
		log.Fatalf("Unable to open: %v\n", err)
	}

	/* Set veth1 of the new container to the new network namespace */
	veth1 := "veth1_" + containerId[:6]
	veth1Link, err := netlink.LinkByName(veth1)
	if err != nil {
		log.Fatalf("Unable to fetch veth1: %v\n", err)
	}
	if err := netlink.LinkSetNsFd(veth1Link, fd); err != nil {
		log.Fatalf("Unable to set network namespace for veth1: %v\n", err)
	}

	if err := unix.Setns(fd, unix.CLONE_NEWNET); err != nil {
		log.Fatalf("Setns system call failed: %v\n", err)
	}

	addr, _ := netlink.ParseAddr(createIPAddress() + "/16")
	if err := netlink.AddrAdd(veth1Link, addr); err != nil {
		log.Fatalf("Error assigning IP to veth1: %v\n", err)
	}

	// Bring up the interface
	utils.DoOrDieWithMessage(netlink.LinkSetUp(veth1Link), "Unable to bring up veth1")

	// Add a default route
	route := netlink.Route{
		Scope:      netlink.SCOPE_UNIVERSE,
		ILinkIndex: veth1Link.Attrs().Index,
		Gw:         net.ParseIP("172.29.0.1"),
		Dst:        nil,
	}
	utils.DoOrDieWithMessage(netlink.RouteAdd(&route), "Unable to add default route")
}

// Go through the list of interfaces and return true if the container0 bridge is up
func IsContainerBridgeUp() (bool, error) {
	if links, err := netlink.LinkList(); err != nil {
		log.Printf("Unable to get list of links.\n")
		return false, err
	} else {
		for _, link := range links {
			if link.Type() == "bridge" && link.Attrs().Name == "container0" {
				log.Printf("IsContainerBridgeUp exist true.\n")
				return true, nil
			}
		}
		return false, err
	}
}

/*
	This function sets up the "container0" bridge, which is our main bridge
	interface. To keep things simple, we assign the hopefully unassigned
	and obscure private IP 172.29.0.1 to it, which is from the range of
	IPs which we will also use for our containers.
*/
func SetupContainerBridge() error {
	linkAttrs := netlink.NewLinkAttrs()
	linkAttrs.Name = "container0"
	containerBridge := &netlink.Bridge{LinkAttrs: linkAttrs}
	if err := netlink.LinkAdd(containerBridge); err != nil {
		return err
	}
	address, _ := netlink.ParseAddr("172.29.0.1/16")
	netlink.AddrAdd(containerBridge, address)
	netlink.LinkSetUp(containerBridge)

	return nil
}

func JoinContainerNetworkNamespace(containerId string) error {
	nsMount := nsMountBase + "/" + containerId
	fd, err := unix.Open(nsMount, unix.O_RDONLY, 0)
	if err != nil {
		log.Printf("Unable to open: %v\n", err)
		return err
	}

	if err := unix.Setns(fd, unix.CLONE_NEWNET); err != nil {
		log.Printf("Setns system call failed: %v\n", err)
		return err
	}

	return nil
}

/*
	This is the function that sets the IP address for the local interface.
	There seems to be a bug in the netlink library in that it does not
	succeed in looking up the local interface by name, always returning an
	error. As a workaround, we loop through the interfaces, compare the name,
	set the IP and make the interface up.
*/
func SetupLocalInterface() {
	links, _ := netlink.LinkList()
	for _, link := range links {
		if link.Attrs().Name == "lo" {
			loAddr, _ := netlink.ParseAddr("127.0.0.1/32")
			if err := netlink.AddrAdd(link, loAddr); err != nil {
				log.Println("Unable to configure local interface!")
			}
			netlink.LinkSetUp(link)
		}
	}
}
