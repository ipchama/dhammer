![alt text](sledge.jpg "Dhammer")
[![DeepSource](https://static.deepsource.io/deepsource-badge-light.svg)](https://deepsource.io/gh/ipchama/dhammer/?ref=repository-badge)
# Dhammer

Dhammer is a stress-tester for DHCP servers.  It currently only supports DHCPv4, but it was created with the strong intention of including DHCPv6 in the very near future.

**Please, read the full disclaimer at the bottom of this document**.

**This tool is meant for testing YOUR OWN networks and servers**.  I wrote this with the intention of using it to stress-test a DHCP framework I have in development and to aid in the addition of DHCPv6 features to the framework.  Thus, Dhammer is meant to be used as a diagnostic tool, and it should not be used for malicious activity.

Dhammer can act as a "local" DHCP client broadcasting packets, or it can simulate a DHCP relay, allowing you to test DHCP servers outside of your local network and also to avoid any potential broadcast-storm safeguards your router might have.

It's also possible to have dhammer bind any assigned IPs to the loopback and handle ARP requests, including responding to ARPs with a generated MAC address instead of the actual MAC of the interface sending the original DHCP requests.

## Getting Started

Just download, compile, and run.

### Installing

```
go get -u github.com/ipchama/dhammer
```
### Building
```
go build .
```
## Examples
#### Broadcast on the local network 
```
sudo ./dhammer dhcpv4 --interface wlan1 --mac-count 10000 --rps 100 --maxlife 0
```
#### Target a specific server via DHCP relay
```
sudo ./dhammer dhcpv4 --interface wlan1 --mac-count 10000 --gateway-mac "48:f8:b6:f7:30:28" --rps 1000 --maxlife 0 --relay-target-server-ip 192.168.1.1 --relay-source-ip 192.168.1.143
```
To use the relay, particularly if you'll be attempting to test a server across the WAN, you'll need to pass in the MAC of your gateway, which can easily be obtained by checking your ARP table (Ex: `arp -a -n`).  I hope to have gateway MAC detection become automatic in a future release.

Dhammer uses very raw sockets to do its job, so `CAP_NET_ADMIN` and `CAP_NET_RAW` are needed at the very least.  I.e., just `sudo` and get moving.

Stats are now accessible via API calls with JSON responses.  An example python script to interact with them is included in the repo.

Example response from http://localhost:8080/stats:
```
[
  {
    "stat_name": "DiscoverSent",
    "stat_value": 1066,
    "stat_previous_ticker_value": 975,
    "stat_rate_per_second": 65
  },
  {
    "stat_name": "InfoSent",
    "stat_value": 0,
    "stat_previous_ticker_value": 0,
    "stat_rate_per_second": 0
  },
  {
    "stat_name": "RequestSent",
    "stat_value": 933,
    "stat_previous_ticker_value": 846,
    "stat_rate_per_second": 63.333333333333336
  },
  {
    "stat_name": "DeclineSent",
    "stat_value": 0,
    "stat_previous_ticker_value": 0,
    "stat_rate_per_second": 0
  },
  {
    "stat_name": "ReleaseSent",
    "stat_value": 0,
    "stat_previous_ticker_value": 0,
    "stat_rate_per_second": 0
  },
  {
    "stat_name": "OfferReceived",
    "stat_value": 933,
    "stat_previous_ticker_value": 846,
    "stat_rate_per_second": 63.333333333333336
  },
  {
    "stat_name": "AckReceived",
    "stat_value": 882,
    "stat_previous_ticker_value": 802,
    "stat_rate_per_second": 61
  },
  {
    "stat_name": "NakReceived",
    "stat_value": 0,
    "stat_previous_ticker_value": 0,
    "stat_rate_per_second": 0
  }
]
```
## Contributing

Contributions are welcome.  In particular, help me make the stats better! :D

## License

This project is licensed under the GPL v3 License - see the [LICENSE.md](LICENSE.md) file for details

## Disclaimer

BY USING AND/OR UTILIZING THE CODE FROM THIS REPOSITORY IN ANY FORM, COMPILED BINARY FORM OR OTHERWISE, YOU HEREBY ASSUME ALL OF THE RISK IN ANY AND ALL ACTIVITIES ASSOCIATED WITH ANY USE, including by way of example and not limitation, any risks that may arise from negligence or carelessness on the part of any persons or entities making use of the content of this repository for any purpose. 

By making use of the content of this repository you agree to the following:

( A ) YOU WILL NOT use the content of this repository in any form, compiled or otherwise, for malicious activity.

( B ) INDEMNIFY AND HOLD HARMLESS the creators and maintainers of all code in this repository from any and all liabilities or claims made as a result of using the content of this repository in any activity. 

I acknowledge that the creators and maintainers of this repository are NOT responsible for the errors, omissions, acts, or failures to act of any party or entity conducting any activity that makes use of the content of this repository. 

This Release of Liability waiver shall be construed broadly to provide a release and waiver to the maximum extent permissible under applicable law. 
BY MAKING USE OF ANY CONTENT OF THIS REPOSITORY AND/OR PROJECT, YOU CERTIFY THAT YOU HAVE READ THIS DOCUMENT AND YOU FULLY UNDERSTAND ITS CONTENT.


