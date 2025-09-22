# Counter

Fast, scalable, simple to deploy metrics server that tracks sum, count, and average for any arbitrary metrics that you send to it.

## Installation

Download the binary for your platform from the [bin directory](../master/bin) to the location where you want it
to store data, then run it as an unpriviledged user. It will run on port 8080 by default.

## User interface

If you visit the [server root](http://localhost:8080/), you will see a summary of all metrics being tracked
by the server.

![Image](../master/examples/ui.png?raw=true)

## API endpoints

### GET /metric

List metrics that are currently being tracked

### GET /metric/:name

Get a JSON representation of all samples currently being tracked for this metric

### GET /metric/:name/:operation.png

Get a PNG representation of this metric, where operation is "sum", "count", or "avg"

![Image](../master/examples/avg.png?raw=true)

### POST /metric

Send metrics to be tracked. Any unrecognized metrics will be created automatically.
The API format is as follows:

    {
        "metric.name": {
            "count": 100,
            "value": 86753.09
        }
    }

Multiple metrics can be sent in a single request, and you can aggregate upstream by including a
count in your request. Count is optional, and will default to 1.

## Command line options

| Flag   | Description                         |
| ------ | ----------------------------------- |
| --port | Specify the port on which to listen |

## Purging data

If you no longer wish to track a metric, stop the server, delete its datafile in the data directory,
then restart the server.

## Installing systemd service so it starts on boot

Assuming you install counter to /opt/counter on Ubuntu, you can download the counter.example.service
and run the following:

    sudo mv counter.example.service /lib/systemd/system/counter.service
    sudo systemctl start counter
