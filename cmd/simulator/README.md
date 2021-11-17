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
2021/11/17 14:45:46 flags: {Filename:configs/SpeedwayR-12-34-FF.json Silent:false LLRPPort:5084 APIPort:55084 AntennaCount:2 TagPopulation:20 BaseEPC:301400000000000000000000 MaxRSSI:-60 MinRSSI:-80 ReadRate:100}
2021/11/17 14:45:46 Loading simulator config from 'configs/SpeedwayR-12-34-FF.json'
2021/11/17 14:45:46 Successfully loaded simulator config.
2021/11/17 14:45:46 Starting llrp simulator on port: 5084
2021/11/17 14:45:46 Serving API @ http://localhost:55084
```

At this point, the llrp simulated device is up and running waiting for connections.

## LLRP Device Service
This simulator is intended to be used with the [EdgeX Device RFID LLRP Go](https://github.com/edgexfoundry/device-rfid-llrp-go).

After running the simulator, you should be able to discover it via the device service.

## Usage
```
Usage of simulator:
  -M int
        max rssi (default -60)
  -P int
        api port (default 55084)
  -a int
        antenna count (default 2)
  -antenna-count int
        antenna count (default 2)
  -api-port int
        api port (default 55084)
  -base-epc string
        Base EPC (default "301400000000000000000000")
  -e string
        Base EPC (default "301400000000000000000000")
  -epc string
        Base EPC (default "301400000000000000000000")
  -f string
        config filename
  -file string
        config filename
  -llrp-port int
        llrp port (default 5084)
  -m int
        min rssi (default -80)
  -max int
        max rssi (default -60)
  -max-rssi int
        max rssi (default -60)
  -min int
        min rssi (default -80)
  -min-rssi int
        min rssi (default -80)
  -p int
        llrp port (default 5084)
  -port int
        llrp port (default 5084)
  -r int
        read rate (tags/s) (default 100)
  -rate int
        read rate (tags/s) (default 100)
  -read-rate int
        read rate (tags/s) (default 100)
  -s    silent
  -silent
        silent
  -t int
        tag population count (default 20)
  -tag-population int
        tag population count (default 20)
  -tags int
        tag population count (default 20)
```
