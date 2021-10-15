package go-arping

import (
	"errors"
	"fmt"
	"github.com/j-keck/arping"
	"net"
	"os/exec"
	"regexp"
	"time"
)

const (
	DefaultRetryTimes = 10
	MacRegRUle = "([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})"
)

func sendArp(ip net.IP) (net.HardwareAddr, error) {
	if hwAddr, duration, err := arping.Ping(ip); err != nil {
		return nil ,err
	} else {
		return hwAddr, nil
	}
}

func ArPingCmd(ip net.IP, inter net.Interface) (net.HardwareAddr, error) {
	command := exec.Command("arping", "-f", "-I", inter.Name, ip.String())
	ch := make(chan bool, 1)
	var outputStr string
	go func() {
		out, _ := command.Output()
		outputStr = string(out)
		ch <- true
		close(ch)
	}()
	select {
		case <- ch:
			macReg := regexp.MustCompile(MacRegRUle)
			regResult := macReg.FindAllString(outputStr, -1)
			if len(regResult) != 0 {
				macHardAddr, transError := net.ParseMAC(regResult[0])
				if transError == nil{
					return macHardAddr, nil
				}
			}
			errStr := fmt.Sprintf("ArPing %v faild command: %v",ip.String(), command.Args)
			return nil, errors.New(errStr)
		case <- time.After(3 * time.Second):
			return  nil, errors.New("cmd time out")
	}
}


func retrySendArp(ip net.IP)  (net.HardwareAddr, error){
	var attempts = DefaultRetryTimes
	for attempts > 0 {
		mac, err := sendArp(ip)
		if err != nil {
			attempts--
			continue
		}
		return mac, nil
	}
	inter, err := getInterfaceWithIp(ip)
	if err != nil {
		return nil, err
	}
	mac, err := ArPingCmd(ip, *inter)
	if err == nil {
		return mac, nil
	}
	errorStr := fmt.Sprintf("Get arp mac address failed after retry %v times", DefaultRetryTimes + 1)
	return nil, errors.New(errorStr)
}


func getInterfaceWithIp(dstIP net.IP) (*net.Interface, error){
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	isDown := func(iface net.Interface) bool {
		return iface.Flags&1 == 0
	}
	hasAddressInNetwork := func(iface net.Interface) bool {
		if _, err := findIPInNetworkFromIface(dstIP, iface); err != nil {
			return false
		}
		return true
	}
	for _, iface := range ifaces {
		if isDown(iface) {
			continue
		}
		if !hasAddressInNetwork(iface) {
			continue
		}
		return &iface, nil
	}
	return nil, errors.New("no usable interface found")
}

func findIPInNetworkFromIface(dstIP net.IP, iface net.Interface) (net.IP, error) {
	addrs, err := iface.Addrs()

	if err != nil {
		return nil, err
	}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok {
			if ipnet.Contains(dstIP) {
				return ipnet.IP, nil
			}
		}
	}
	return nil, fmt.Errorf("iface: '%s' can't reach ip: '%s'", iface.Name, dstIP)
}
