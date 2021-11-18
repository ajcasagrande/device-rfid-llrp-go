package llrp

import (
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	DefaultDevicePrefix = "LLRP"
)

func decodeReaderSuffix(id *Identification, format string) string {
	rID := id.ReaderID
	if id.IDType == ID_MAC_EUI64 && len(rID) >= 3 {
		mac := rID[len(rID)-3:]
		return fmt.Sprintf(format, mac[0], mac[1], mac[2])
	}
	return hex.EncodeToString(rID)
}

func determineImpinjName(model ImpinjModelType, id *Identification) string {
	return fmt.Sprintf("%s-%s",
		model.HostnamePrefix(DefaultDevicePrefix), decodeReaderSuffix(id, "%02X-%02X-%02X"))
}

func determineZebraName(model ZebraModelType, id *Identification) string {
	return fmt.Sprintf("%s%s",
		model.HostnamePrefix(DefaultDevicePrefix), decodeReaderSuffix(id, "%02x%02x%02x"))
}

func determineGenericName(id *Identification) string {
	return fmt.Sprintf("%s-%s",
		DefaultDevicePrefix, decodeReaderSuffix(id, "%02X-%02X-%02X"))
}

func DetermineVendorAndModelName(vendor, model uint32) (string, string) {
	vName := VendorIDType(vendor).String()
	switch VendorIDType(vendor) {
	case Impinj:
		return vName, ImpinjModelType(model).String()
	case Zebra:
		return vName, strings.ReplaceAll(ZebraModelType(model).String(), "_", "-")
	default:
		return vName, fmt.Sprintf("UnknownModel(%d)", model)
	}
}

func DetermineDeviceName(vendor, model uint32, id *Identification) string {
	switch VendorIDType(vendor) {
	case Impinj:
		return determineImpinjName(ImpinjModelType(model), id)
	case Zebra:
		return determineZebraName(ZebraModelType(model), id)
	default:
		return determineGenericName(id)
	}
}
