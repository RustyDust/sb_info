package main

import (
	"flag"
	"fmt"
	"sb_info/sb"
)

func main() {
	// parse the command line
	u := flag.String("u", "", "username, one of `Vendor`, `Installer`, `Service`, `User`, `Oem`")
	p := flag.String("p", "", "password")
	h := flag.String("h", "", "host")
	flag.Parse()
	flag.Args()

	fmt.Println("SonnenBatterie Check")
	fmt.Println("====================\n")

	fmt.Printf("Using:\nUser: User\nPass: %s\nHost: %s\n\n", *p, *h)

	sunBat := sb.SunBatInit(*u, *p, *h)
	sunBat.Login()

	// get the Data
	res, ok := sunBat.Get("battery_system")
	if !ok {
		fmt.Println("!!! - unable to get SonnenBatterie system information\n")
	}
	fmt.Printf("System:\n%s\n\n", res)

	res, ok = sunBat.Get("powermeter")
	if !ok {
		fmt.Println("!!! - unable to get SonnenBatterie power meter information\n")
	}
	fmt.Printf("Power meter:\n%s\n\n", res)

	res, ok = sunBat.Get("inverter")
	if !ok {
		fmt.Println("!!! - unable to get SonnenBatterie inverter information\n")
	}
	fmt.Printf("Inverter:\n%s\n\n", res)

	res, ok = sunBat.Get("system_data")
	if !ok {
		fmt.Println("!!! - unable to get SonnenBatterie system data information\n")
	}
	fmt.Printf("System data:\n%s\n\n", res)

	res, ok = sunBat.Get("v1/status")
	if !ok {
		fmt.Println("!!! - unable to get SonnenBatterie status information\n")
	}
	fmt.Printf("Status:\n%s\n\n", res)

	res, ok = sunBat.Get("battery")
	if !ok {
		fmt.Println("!!! - unable to get SonnenBatterie battery information\n")
	}
	fmt.Printf("Battery:\n%s\n\n", res)

}
