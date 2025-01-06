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
	t := flag.Bool("2", false, "Use only Api v2")
	flag.Parse()
	flag.Args()

	fmt.Println("SonnenBatterie Check")
	fmt.Println("====================")

	fmt.Printf("\nUsing:\nUser: User\nPass: %s\nHost: %s\n\n", *p, *h)

	sunBat := sb.SunBatInit(*u, *p, *h)
	sunBat.Login()

	// get the Data (Api v1)
	if *t {
		res, ok := sunBat.GetApi2("configurations")
		if !ok {
			fmt.Println("!!! - unable to get SonnenBatterie configurations (API v2)")
		}
		fmt.Printf("SonnenBatterie Configurations:\n%v\n\n", res)

		res, ok = sunBat.GetApi2("battery")
		if !ok {
			fmt.Println("!!! - unable to get SonnenBatterie battery module data (API v2)")
		}
		fmt.Printf("SonnenBatterie Battery Module Data:\n%v\n\n", res)

		res, ok = sunBat.GetApi2("inverter")
		if !ok {
			fmt.Println("!!! - unable to get SonnenBatterie inverter data (API v2)")
		}
		fmt.Printf("SonnenBatterie Battery Inverter Data:\n%v\n\n", res)

		res, ok = sunBat.GetApi2("latestdata")
		if !ok {
			fmt.Println("!!! - unable to get SonnenBatterie latest data (API v2)")
		}
		fmt.Printf("SonnenBatterie Battery Latest Data:\n%v\n\n", res)

		res, ok = sunBat.GetApi2("powermeter")
		if !ok {
			fmt.Println("!!! - unable to get SonnenBatterie powermeter measurements (API v2)")
		}
		fmt.Printf("SonnenBatterie Powermeter Measurements:\n%v\n\n", res)

		res, ok = sunBat.GetApi2("status")
		if !ok {
			fmt.Println("!!! - unable to get SonnenBatterie status (API v2)")
		}
		fmt.Printf("SonnenBatterie Status:\n%v\n\n", res)

	} else {
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

}
