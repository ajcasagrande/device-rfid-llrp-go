# LLRP Simulator

> **Important Note:** This simulator has been designed to provide very basic
> functionality and is nowhere near compliant with the LLRP Spec.

## TL:DR
Run this inside a terminal:
```shell
cd cmd/simulator
go run simulator.go -f configs/SpeedwayR-12-34-FF.json
```

## Setup
In order to run the simulator, you must provide `-f/-file <reader.json>`, where
`reader.json` is a json file containing both `ReaderCapabilities` and `ReaderConfig` like so:
```json
{
  "ReaderCapabilities": {
    ...
  },
  "ReaderConfig": {
    ...
  }
}
```

The values of these two fields, are json-encoded responses from the
`GetReaderCapabilities` and `GetReaderConfig` LLRP calls (json encoded from this library).

An example of this file can be found at [configs/SpeedwayR-12-34-FF.json](configs/SpeedwayR-12-34-FF.json)

## Run
```shell
cd cmd/simulator
go run simulator.go -f configs/SpeedwayR-12-34-FF.json
```

If you would like to run more than one device at a time, you can do this by changing
the port it listens on for LLRP messages using `-p <int> / -port <int> / -llrp-port <int>`.

You may also need to override the API port using `-P <int> / -api-port <int>`.

### Example Output
When run, you should see messages like so:
```
2021/11/18 12:05:38 flags: {Filename:configs/SpeedwayR-12-34-FF.json Silent:false LLRPPort:5084 APIPort:0 AntennaCount:2 TagPopulation:20 BaseEPC:301400000000000000000000 MaxRSSI:-60 MinRSSI:-80 ReadRate:100}
2021/11/18 12:05:38 Loading simulator config from 'configs/SpeedwayR-12-34-FF.json'
2021/11/18 12:05:38 Successfully loaded simulator config.
2021/11/18 12:05:38 ***** Simulated Device Information *****
2021/11/18 12:05:38   Device Name     : SpeedwayR-12-34-FF
2021/11/18 12:05:38   Manufacturer    : Impinj
2021/11/18 12:05:38   Model           : SpeedwayR420
2021/11/18 12:05:38   FW Version      : 5.14.0.240
2021/11/18 12:05:38 ****************************************
2021/11/18 12:05:38 Starting llrp simulator on port 5084
2021/11/18 12:05:38 Listening for LLRP messages on port 5084
```

At this point, the llrp simulated device is up and running waiting for connections.

## LLRP Device Service
This simulator is intended to be used with the [EdgeX Device RFID LLRP Go](https://github.com/edgexfoundry/device-rfid-llrp-go).

After running the simulator, you should be able to discover it via the device service.

## Usage
```
Usage of llrp-simulator:
  -a / -antenna-count <int>
        antenna count (default 2)
        
  -P / -api-port <int>
        api port (default 55084)

  -e / -epc / -base-epc <string>
        Base EPC (default "301400000000000000000000")

  -f / -file <string>
        config filename

  -M / -max / -max-rssi <int>
        max rssi (default -60)

  -m / -min / -min-rssi <int>
        min rssi (default -80)

  -p / -port / -llrp-port <int>
        llrp port (default 5084)

  -r / -rate / -read-rate <int>
        read rate (tags/s) (default 100)

  -t / -tags / -tag-population <int>
        tag population count (default 20)

  -s / -silent
        silent
```

## Docker
### Build
To build the docker image, run:
```shell
make docker
```

### Run
To run the docker image, run:
> Note: Substitute `<VERSION>` with the version tagged during `make build`
```shell
docker run -it --rm -p "5084:5084" llrp-sim:<VERSION> \
    -f configs/SpeedwayR-12-34-FF.json
```
