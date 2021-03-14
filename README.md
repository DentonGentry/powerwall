# Utilities to manage and monitor a Tesla Powerwall

Our home is built on a hill, which severely curtails solar production during the winter
months. The sun drops below the hilltop shortly after noon, production in summer is 8X
higher than in December/January.

By default, a Tesla Powerwall will charge itself using surplus solar production after
first powering the house. In our case this led to poor results, as solar production in
winter never charged the Powerwall and therefore we paid for substantial Peak rate power
every day. Financially it is better for us to charge the Powerwall during the day and let
it power the house as much as it can during Peak hours.

The utilities in this project help in managing and monitoring a Tesla Powerwall for
situations like this.

### cmd/powerwall
A command line utility to set the charge percentage. We run it from cron on a Linux
system. During winter months, at 6am we set the charge percentage to 100% to make the
powerwall capture as much of the solar production as it can. At 3pm (partial peak)
we set it to hold its current charge, and allow any remaining solar production to power
the house. At 4pm we set the powerwall charge percentage to a lower value, allowing it
to start powering the house during Peak hours.

The goal is to charge the Powerwall up to close to 100% each day. One might consider that
letting the Powerwall vary between 20% and 60% provides the same financial benefit as
60% to 100%, however in case of power outage having 60% power available is preferable.

This utility communicates with Tesla's cloud service to control the Powerwall, so it
requires the same username and password as used by the Tesla app on a phone.


### cmd/powerwall\_prometheus
A daemon to poll the the amount of solar, battery, and house demand every few seconds and
export them on /metrics for [Prometheus](https://prometheus.io/) to monitor. Because it
polls so frequently, it queries the local Backup Gateway directly and does not rely on
Tesla's cloud service.


### cmd/solcast\_uploader
A utility intended to run from cron at the end of the day, extracting production information
from Prometheus to upload to solcast.com.au. A future goal is to use day-ahead forecasts
from Solcast to determine how much to allow the Powerwall to discharge.


## How to get started
All of these utilities are written in Go. I use Go 1.15, though earlier or later versions of
the toolchain may work. To build all of the utilities: `go build ./...`

The binaries can be copied to /usr/local/bin:
`sudo cp cmd/powerwall/powerwall cmd/powerwall_prometheus/powerwall_prometheus /usr/local/bin`

cmd/powerwall is intended to run from cron. There is an example\_crontab.txt file in that
directory showing how we use it.

cmd/powerwall\_prometheus is run as a daemon. There is a systemctl script in that directory
showing how we use it.


## License
All code in this project is licensed under a 3-clause BSD license, as detailed in the LICENSE
file in the top level directory.
