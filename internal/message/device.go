package message

// DeviceNamer provides a method to get a device name from a device index.
type DeviceNamer interface {
	DeviceName(index int) (name string, ok bool)
}
