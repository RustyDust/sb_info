# sb_test
Utility to read the information from a SonnenBatterie via the provided APIs.

## Usage
Download the version for your operating system from the [Releases](https://github.com/RustyDust/sb_info/releases) page,
unpack and then in a terminal run:

### Mac / Linux
``` bash
./sb_info -u <user type> -p <password> -h <ip/host>
```

### Windows
``` powershell
./sb_info.exe -u <user type> -p <password> -h <ip/host>
```

#### Parameters
- `<user type>` := String, one of `Vendor`, `Installer`, `Service`, `User`, `Oem`
- `<password` := String, your password either as set by you or provided by Sonnen
- `<isp/host>` := String, IP address or host name of the SonnenBatterie

#### Examples

``` bash
### Mac/Linux using the `User` user type
./sb_info -u User -p SecretPassword -h 192.168.0.22

### Windows using the `Vendor` user type
.\sb_info.exe -u Vendor -p VendorSecret -h 172.16.17.18    
```
