/*
// Copyright (c) 2016 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
*/

// Package qemu provides methods and types for launching and managing QEMU
// instances.  Instances can be launched with the LaunchQemu function and
// managed thereafter via QMPStart and the QMP object that this function
// returns.  To manage a qemu instance after it has been launched you need
// to pass the -qmp option during launch requesting the qemu instance to create
// a QMP unix domain manageent socket, e.g.,
// -qmp unix:/tmp/qmp-socket,server,nowait.  For more information see the
// example below.
package qemu

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"context"
)

// Machine describes the machine type qemu will emulate.
type Machine struct {
	// Type is the machine type to be used by qemu.
	Type string

	// Acceleration are the machine acceleration options to be used by qemu.
	Acceleration string
}

// Device is the qemu device interface.
type Device interface {
	Valid() bool
	QemuParams(config *Config) []string
}

// DeviceDriver is the device driver string.
type DeviceDriver string

const (
	// NVDIMM is the Non Volatile DIMM device driver.
	NVDIMM DeviceDriver = "nvdimm"

	// Virtio9P is the 9pfs device driver.
	Virtio9P = "virtio-9p-pci"

	// VirtioNet is the virt-io networking device driver.
	VirtioNet = "virtio-net"

	// VirtioNetPCI is the virt-io pci networking device driver.
	VirtioNetPCI = "virtio-net-pci"

	// VirtioSerial is the serial device driver.
	VirtioSerial = "virtio-serial-pci"

	// VirtioBlock is the block device driver.
	VirtioBlock = "virtio-blk"

	// Console is the console device driver.
	Console = "virtconsole"

	// VirtioSerialPort is the serial port device driver.
	VirtioSerialPort = "virtserialport"
)

// ObjectType is a string representing a qemu object type.
type ObjectType string

const (
	// MemoryBackendFile represents a guest memory mapped file.
	MemoryBackendFile ObjectType = "memory-backend-file"
)

// Object is a qemu object representation.
type Object struct {
	// Driver is the qemu device driver
	Driver DeviceDriver

	// Type is the qemu object type.
	Type ObjectType

	// ID is the user defined object ID.
	ID string

	// DeviceID is the user defined device ID.
	DeviceID string

	// MemPath is the object's memory path.
	// This is only relevant for memory objects
	MemPath string

	// Size is the object size in bytes
	Size uint64
}

// Valid returns true if the Object structure is valid and complete.
func (object Object) Valid() bool {
	switch object.Type {
	case MemoryBackendFile:
		if object.ID == "" || object.MemPath == "" || object.Size == 0 {
			return false
		}

	default:
		return false
	}

	return true
}

// QemuParams returns the qemu parameters built out of this Object device.
func (object Object) QemuParams(config *Config) []string {
	var objectParams []string
	var deviceParams []string
	var qemuParams []string

	deviceParams = append(deviceParams, string(object.Driver))
	deviceParams = append(deviceParams, fmt.Sprintf(",id=%s", object.DeviceID))

	switch object.Type {
	case MemoryBackendFile:
		objectParams = append(objectParams, string(object.Type))
		objectParams = append(objectParams, fmt.Sprintf(",id=%s", object.ID))
		objectParams = append(objectParams, fmt.Sprintf(",mem-path=%s", object.MemPath))
		objectParams = append(objectParams, fmt.Sprintf(",size=%d", object.Size))

		deviceParams = append(deviceParams, fmt.Sprintf(",memdev=%s", object.ID))
	}

	qemuParams = append(qemuParams, "-device")
	qemuParams = append(qemuParams, strings.Join(deviceParams, ""))

	qemuParams = append(qemuParams, "-object")
	qemuParams = append(qemuParams, strings.Join(objectParams, ""))

	return qemuParams
}

// FSDriver represents a qemu filesystem driver.
type FSDriver string

// SecurityModelType is a qemu filesystem security model type.
type SecurityModelType string

const (
	// Local is the local qemu filesystem driver.
	Local FSDriver = "local"

	// Handle is the handle qemu filesystem driver.
	Handle = "handle"

	// Proxy is the proxy qemu filesystem driver.
	Proxy = "proxy"
)

const (
	// None is like passthrough without failure reports.
	None SecurityModelType = "none"

	// PassThrough uses the same credentials on both the host and guest.
	PassThrough = "passthrough"

	// MappedXattr stores some files attributes as extended attributes.
	MappedXattr = "mapped-xattr"

	// MappedFile stores some files attributes in the .virtfs directory.
	MappedFile = "mapped-file"
)

// FSDevice represents a qemu filesystem configuration.
type FSDevice struct {
	// Driver is the qemu device driver
	Driver DeviceDriver

	// FSDriver is the filesystem driver backend.
	FSDriver FSDriver

	// ID is the filesystem identifier.
	ID string

	// Path is the host root path for this filesystem.
	Path string

	// MountTag is the device filesystem mount point tag.
	MountTag string

	// SecurityModel is the security model for this filesystem device.
	SecurityModel SecurityModelType
}

// Valid returns true if the FSDevice structure is valid and complete.
func (fsdev FSDevice) Valid() bool {
	if fsdev.ID == "" || fsdev.Path == "" || fsdev.MountTag == "" {
		return false
	}

	return true
}

// QemuParams returns the qemu parameters built out of this filesystem device.
func (fsdev FSDevice) QemuParams(config *Config) []string {
	var fsParams []string
	var deviceParams []string
	var qemuParams []string

	deviceParams = append(deviceParams, fmt.Sprintf("%s", fsdev.Driver))
	deviceParams = append(deviceParams, fmt.Sprintf(",fsdev=%s", fsdev.ID))
	deviceParams = append(deviceParams, fmt.Sprintf(",mount_tag=%s", fsdev.MountTag))

	fsParams = append(fsParams, string(fsdev.FSDriver))
	fsParams = append(fsParams, fmt.Sprintf(",id=%s", fsdev.ID))
	fsParams = append(fsParams, fmt.Sprintf(",path=%s", fsdev.Path))
	fsParams = append(fsParams, fmt.Sprintf(",security_model=%s", fsdev.SecurityModel))

	qemuParams = append(qemuParams, "-device")
	qemuParams = append(qemuParams, strings.Join(deviceParams, ""))

	qemuParams = append(qemuParams, "-fsdev")
	qemuParams = append(qemuParams, strings.Join(fsParams, ""))

	return qemuParams
}

// CharDeviceBackend is the character device backend for qemu
type CharDeviceBackend string

const (
	// Pipe creates a 2 way connection to the guest.
	Pipe CharDeviceBackend = "pipe"

	// Socket creates a 2 way stream socket (TCP or Unix).
	Socket = "socket"

	// CharConsole sends traffic from the guest to QEMU's standard output.
	CharConsole = "console"

	// Serial sends traffic from the guest to a serial device on the host.
	Serial = "serial"

	// TTY is an alias for Serial.
	TTY = "tty"

	// PTY creates a new pseudo-terminal on the host and connect to it.
	PTY = "pty"
)

// CharDevice represents a qemu character device.
type CharDevice struct {
	Backend CharDeviceBackend

	// Driver is the qemu device driver
	Driver DeviceDriver

	// Bus is the serial bus associated to this device.
	Bus string

	// DeviceID is the user defined device ID.
	DeviceID string

	ID   string
	Path string
	Name string
}

// Valid returns true if the CharDevice structure is valid and complete.
func (cdev CharDevice) Valid() bool {
	if cdev.ID == "" || cdev.Path == "" {
		return false
	}

	return true
}

// QemuParams returns the qemu parameters built out of this character device.
func (cdev CharDevice) QemuParams(config *Config) []string {
	var cdevParams []string
	var deviceParams []string
	var qemuParams []string

	deviceParams = append(deviceParams, fmt.Sprintf("%s", cdev.Driver))
	if cdev.Bus != "" {
		deviceParams = append(deviceParams, fmt.Sprintf(",bus=%s", cdev.Bus))
	}
	deviceParams = append(deviceParams, fmt.Sprintf(",chardev=%s", cdev.ID))
	deviceParams = append(deviceParams, fmt.Sprintf(",id=%s", cdev.DeviceID))
	if cdev.Name != "" {
		deviceParams = append(deviceParams, fmt.Sprintf(",name=%s", cdev.Name))
	}

	cdevParams = append(cdevParams, string(cdev.Backend))
	cdevParams = append(cdevParams, fmt.Sprintf(",id=%s", cdev.ID))
	if cdev.Backend == Socket {
		cdevParams = append(cdevParams, fmt.Sprintf(",path=%s,server,nowait", cdev.Path))
	} else {
		cdevParams = append(cdevParams, fmt.Sprintf(",path=%s", cdev.Path))
	}

	qemuParams = append(qemuParams, "-device")
	qemuParams = append(qemuParams, strings.Join(deviceParams, ""))

	qemuParams = append(qemuParams, "-chardev")
	qemuParams = append(qemuParams, strings.Join(cdevParams, ""))

	return qemuParams
}

// NetDeviceType is a qemu networing device type.
type NetDeviceType string

const (
	// TAP is a TAP networking device type.
	TAP NetDeviceType = "tap"

	// MACVTAP is a MAC virtual TAP networking device type.
	MACVTAP = "macvtap"
)

// NetDevice represents a guest networking device
type NetDevice struct {
	// Type is the netdev type (e.g. tap).
	Type NetDeviceType

	// Driver is the qemu device driver
	Driver DeviceDriver

	// ID is the netdevice identifier.
	ID string

	// IfName is the interface name,
	IFName string

	// Bus is the bus path name of a PCI device.
	Bus string

	// Addr is the address offset of a PCI device.
	Addr string

	// DownScript is the tap interface deconfiguration script.
	DownScript string

	// Script is the tap interface configuration script.
	Script string

	// FDs represents the list of already existing file descriptors to be used.
	// This is mostly useful for mq support.
	FDs []*os.File

	// VHost enables virtio device emulation from the host kernel instead of from qemu.
	VHost bool

	// MACAddress is the networking device interface MAC address.
	MACAddress string
}

// Valid returns true if the NetDevice structure is valid and complete.
func (netdev NetDevice) Valid() bool {
	if netdev.ID == "" || netdev.IFName == "" {
		return false
	}

	switch netdev.Type {
	case TAP:
		return true
	case MACVTAP:
		return true
	default:
		return false
	}
}

// QemuParams returns the qemu parameters built out of this network device.
func (netdev NetDevice) QemuParams(config *Config) []string {
	var netdevParams []string
	var deviceParams []string
	var qemuParams []string

	deviceParams = append(deviceParams, fmt.Sprintf("%s", netdev.Driver))
	deviceParams = append(deviceParams, fmt.Sprintf(",netdev=%s", netdev.ID))
	deviceParams = append(deviceParams, fmt.Sprintf(",mac=%s", netdev.MACAddress))

	if netdev.Driver == VirtioNetPCI {
		if netdev.Bus != "" {
			deviceParams = append(deviceParams, fmt.Sprintf(",bus=%s", netdev.Bus))
		}

		if netdev.Addr != "" {
			addr, err := strconv.Atoi(netdev.Addr)
			if err == nil && addr >= 0 {
				deviceParams = append(deviceParams, fmt.Sprintf(",addr=%x", addr))
			}
		}
	}

	netdevParams = append(netdevParams, string(netdev.Type))
	netdevParams = append(netdevParams, fmt.Sprintf(",id=%s", netdev.ID))
	netdevParams = append(netdevParams, fmt.Sprintf(",ifname=%s", netdev.IFName))

	if netdev.DownScript != "" {
		netdevParams = append(netdevParams, fmt.Sprintf(",downscript=%s", netdev.DownScript))
	}

	if netdev.Script != "" {
		netdevParams = append(netdevParams, fmt.Sprintf(",script=%s", netdev.Script))
	}

	if len(netdev.FDs) > 0 {
		var fdParams []string

		qemuFDs := config.appendFDs(netdev.FDs)

		for _, fd := range qemuFDs {
			fdParams = append(fdParams, fmt.Sprintf("%d", fd))
		}

		netdevParams = append(netdevParams, fmt.Sprintf(",fds=%s", strings.Join(fdParams, ":")))
	}

	if netdev.VHost == true {
		netdevParams = append(netdevParams, ",vhost=on")
	}

	qemuParams = append(qemuParams, "-device")
	qemuParams = append(qemuParams, strings.Join(deviceParams, ""))

	qemuParams = append(qemuParams, "-netdev")
	qemuParams = append(qemuParams, strings.Join(netdevParams, ""))

	return qemuParams
}

// SerialDevice represents a qemu serial device.
type SerialDevice struct {
	// Driver is the qemu device driver
	Driver DeviceDriver

	// ID is the serial device identifier.
	ID string
}

// Valid returns true if the SerialDevice structure is valid and complete.
func (dev SerialDevice) Valid() bool {
	if dev.Driver == "" || dev.ID == "" {
		return false
	}

	return true
}

// QemuParams returns the qemu parameters built out of this serial device.
func (dev SerialDevice) QemuParams(config *Config) []string {
	var deviceParams []string
	var qemuParams []string

	deviceParams = append(deviceParams, fmt.Sprintf("%s", dev.Driver))
	deviceParams = append(deviceParams, fmt.Sprintf(",id=%s", dev.ID))

	qemuParams = append(qemuParams, "-device")
	qemuParams = append(qemuParams, strings.Join(deviceParams, ""))

	return qemuParams
}

// BlockDeviceInterface defines the type of interface the device is connected to.
type BlockDeviceInterface string

// BlockDeviceAIO defines the type of asynchronous I/O the block device should use.
type BlockDeviceAIO string

// BlockDeviceFormat defines the image format used on a block device.
type BlockDeviceFormat string

const (
	// NoInterface for block devices with no interfaces.
	NoInterface BlockDeviceInterface = "none"

	// SCSI represents a SCSI block device interface.
	SCSI = "scsi"
)

const (
	// Threads is the pthread asynchronous I/O implementation.
	Threads BlockDeviceAIO = "threads"

	// Native is the pthread asynchronous I/O implementation.
	Native = "native"
)

const (
	// QCOW2 is the Qemu Copy On Write v2 image format.
	QCOW2 BlockDeviceFormat = "qcow2"
)

// BlockDevice represents a qemu block device.
type BlockDevice struct {
	Driver    DeviceDriver
	ID        string
	File      string
	Interface BlockDeviceInterface
	AIO       BlockDeviceAIO
	Format    BlockDeviceFormat
	SCSI      bool
	WCE       bool
}

// Valid returns true if the BlockDevice structure is valid and complete.
func (blkdev BlockDevice) Valid() bool {
	if blkdev.Driver == "" || blkdev.ID == "" || blkdev.File == "" {
		return false
	}

	return true
}

// QemuParams returns the qemu parameters built out of this block device.
func (blkdev BlockDevice) QemuParams(config *Config) []string {
	var blkParams []string
	var deviceParams []string
	var qemuParams []string

	deviceParams = append(deviceParams, fmt.Sprintf("%s", blkdev.Driver))
	deviceParams = append(deviceParams, fmt.Sprintf(",drive=%s", blkdev.ID))
	if blkdev.SCSI == false {
		deviceParams = append(deviceParams, ",scsi=off")
	}

	if blkdev.WCE == false {
		deviceParams = append(deviceParams, ",config-wce=off")
	}

	blkParams = append(blkParams, fmt.Sprintf("id=%s", blkdev.ID))
	blkParams = append(blkParams, fmt.Sprintf(",file=%s", blkdev.File))
	blkParams = append(blkParams, fmt.Sprintf(",aio=%s", blkdev.AIO))
	blkParams = append(blkParams, fmt.Sprintf(",format=%s", blkdev.Format))
	blkParams = append(blkParams, fmt.Sprintf(",if=%s", blkdev.Interface))

	qemuParams = append(qemuParams, "-device")
	qemuParams = append(qemuParams, strings.Join(deviceParams, ""))

	qemuParams = append(qemuParams, "-drive")
	qemuParams = append(qemuParams, strings.Join(blkParams, ""))

	return qemuParams
}

// RTCBaseType is the qemu RTC base time type.
type RTCBaseType string

// RTCClock is the qemu RTC clock type.
type RTCClock string

// RTCDriftFix is the qemu RTC drift fix type.
type RTCDriftFix string

const (
	// UTC is the UTC base time for qemu RTC.
	UTC RTCBaseType = "utc"

	// LocalTime is the local base time for qemu RTC.
	LocalTime = "localtime"
)

const (
	// Host is for using the host clock as a reference.
	Host RTCClock = "host"

	// VM is for using the guest clock as a reference
	VM = "vm"
)

const (
	// Slew is the qemu RTC Drift fix mechanism.
	Slew RTCDriftFix = "slew"

	// NoDriftFix means we don't want/need to fix qemu's RTC drift.
	NoDriftFix = "none"
)

// RTC represents a qemu Real Time Clock configuration.
type RTC struct {
	// Base is the RTC start time.
	Base RTCBaseType

	// Clock is the is the RTC clock driver.
	Clock RTCClock

	// DriftFix is the drift fixing mechanism.
	DriftFix RTCDriftFix
}

// Valid returns true if the RTC structure is valid and complete.
func (rtc RTC) Valid() bool {
	if rtc.Clock != "" {
		if rtc.Clock != Host && rtc.Clock != VM {
			return false
		}
	}

	if rtc.DriftFix != "" {
		if rtc.DriftFix != Slew && rtc.DriftFix != NoDriftFix {
			return false
		}
	}

	return true
}

// QMPSocketType is the type of socket used for QMP communication.
type QMPSocketType string

const (
	// Unix socket for QMP.
	Unix QMPSocketType = "unix"
)

// QMPSocket represents a qemu QMP socket configuration.
type QMPSocket struct {
	// Type is the socket type (e.g. "unix").
	Type QMPSocketType

	// Name is the socket name.
	Name string

	// Server tells if this is a server socket.
	Server bool

	// NoWait tells if qemu should block waiting for a client to connect.
	NoWait bool
}

// Valid returns true if the QMPSocket structure is valid and complete.
func (qmp QMPSocket) Valid() bool {
	if qmp.Type == "" || qmp.Name == "" {
		return false
	}

	if qmp.Type != Unix {
		return false
	}

	return true
}

// SMP is the multi processors configuration structure.
type SMP struct {
	// CPUs is the number of VCPUs made available to qemu.
	CPUs uint32

	// Cores is the number of cores made available to qemu.
	Cores uint32

	// Threads is the number of threads made available to qemu.
	Threads uint32

	// Sockets is the number of sockets made available to qemu.
	Sockets uint32
}

// Memory is the guest memory configuration structure.
type Memory struct {
	// Size is the amount of memory made available to the guest.
	// It should be suffixed with M or G for sizes in megabytes or
	// gigabytes respectively.
	Size string

	// Slots is the amount of memory slots made available to the guest.
	Slots uint8

	// MaxMem is the maximum amount of memory that can be made available
	// to the guest through e.g. hot pluggable memory.
	MaxMem string
}

// Kernel is the guest kernel configuration structure.
type Kernel struct {
	// Path is the guest kernel path on the host filesystem.
	Path string

	// Params is the kernel parameters string.
	Params string
}

// Knobs regroups a set of qemu boolean settings
type Knobs struct {
	// NoUserConfig prevents qemu from loading user config files.
	NoUserConfig bool

	// NoDefaults prevents qemu from creating default devices.
	NoDefaults bool

	// NoGraphic completely disables graphic output.
	NoGraphic bool

	// Daemonize will turn the qemu process into a daemon
	Daemonize bool
}

// Config is the qemu configuration structure.
// It allows for passing custom settings and parameters to the qemu API.
type Config struct {
	// Path is the qemu binary path.
	Path string

	// Ctx is not used at the moment.
	Ctx context.Context

	// Name is the qemu guest name
	Name string

	// UUID is the qemu process UUID.
	UUID string

	// CPUModel is the CPU model to be used by qemu.
	CPUModel string

	// Machine
	Machine Machine

	// QMPSockets is a slice of QMP socket description.
	QMPSockets []QMPSocket

	// Devices is a list of devices for qemu to create and drive.
	Devices []Device

	// RTC is the qemu Real Time Clock configuration
	RTC RTC

	// VGA is the qemu VGA mode.
	VGA string

	// Kernel is the guest kernel configuration.
	Kernel Kernel

	// Memory is the guest memory configuration.
	Memory Memory

	// SMP is the quest multi processors configuration.
	SMP SMP

	// GlobalParam is the -global parameter.
	GlobalParam string

	// Knobs is a set of qemu boolean settings.
	Knobs Knobs

	// fds is a list of open file descriptors to be passed to the spawned qemu process
	fds []*os.File

	qemuParams []string
}

// appendFDs append a list of file descriptors to the qemu configuration and
// returns a slice of offset file descriptors that will be seen by the qemu process.
func (config *Config) appendFDs(fds []*os.File) []int {
	var fdInts []int

	oldLen := len(config.fds)

	config.fds = append(config.fds, fds...)

	// The magic 3 offset comes from https://golang.org/src/os/exec/exec.go:
	//     ExtraFiles specifies additional open files to be inherited by the
	//     new process. It does not include standard input, standard output, or
	//     standard error. If non-nil, entry i becomes file descriptor 3+i.
	for i := range fds {
		fdInts = append(fdInts, oldLen+3+i)
	}

	return fdInts
}

func (config *Config) appendName() {
	if config.Name != "" {
		config.qemuParams = append(config.qemuParams, "-name")
		config.qemuParams = append(config.qemuParams, config.Name)
	}
}

func (config *Config) appendMachine() {
	if config.Machine.Type != "" {
		var machineParams []string

		machineParams = append(machineParams, config.Machine.Type)

		if config.Machine.Acceleration != "" {
			machineParams = append(machineParams, fmt.Sprintf(",accel=%s", config.Machine.Acceleration))
		}

		config.qemuParams = append(config.qemuParams, "-machine")
		config.qemuParams = append(config.qemuParams, strings.Join(machineParams, ""))
	}
}

func (config *Config) appendCPUModel() {
	if config.CPUModel != "" {
		config.qemuParams = append(config.qemuParams, "-cpu")
		config.qemuParams = append(config.qemuParams, config.CPUModel)
	}
}

func (config *Config) appendQMPSockets() {
	for _, q := range config.QMPSockets {
		if q.Valid() == false {
			continue
		}

		qmpParams := append([]string{}, fmt.Sprintf("%s:", q.Type))
		qmpParams = append(qmpParams, fmt.Sprintf("%s", q.Name))
		if q.Server == true {
			qmpParams = append(qmpParams, ",server")
			if q.NoWait == true {
				qmpParams = append(qmpParams, ",nowait")
			}
		}

		config.qemuParams = append(config.qemuParams, "-qmp")
		config.qemuParams = append(config.qemuParams, strings.Join(qmpParams, ""))
	}
}

func (config *Config) appendDevices() {
	for _, d := range config.Devices {
		if d.Valid() == false {
			continue
		}

		config.qemuParams = append(config.qemuParams, d.QemuParams(config)...)
	}
}

func (config *Config) appendUUID() {
	if config.UUID != "" {
		config.qemuParams = append(config.qemuParams, "-uuid")
		config.qemuParams = append(config.qemuParams, config.UUID)
	}
}

func (config *Config) appendMemory() {
	if config.Memory.Size != "" {
		var memoryParams []string

		memoryParams = append(memoryParams, config.Memory.Size)

		if config.Memory.Slots > 0 {
			memoryParams = append(memoryParams, fmt.Sprintf(",slots=%d", config.Memory.Slots))
		}

		if config.Memory.MaxMem != "" {
			memoryParams = append(memoryParams, fmt.Sprintf(",maxmem=%s", config.Memory.MaxMem))
		}

		config.qemuParams = append(config.qemuParams, "-m")
		config.qemuParams = append(config.qemuParams, strings.Join(memoryParams, ""))
	}
}

func (config *Config) appendCPUs() {
	if config.SMP.CPUs > 0 {
		var SMPParams []string

		SMPParams = append(SMPParams, fmt.Sprintf("%d", config.SMP.CPUs))

		if config.SMP.Cores > 0 {
			SMPParams = append(SMPParams, fmt.Sprintf(",cores=%d", config.SMP.Cores))
		}

		if config.SMP.Threads > 0 {
			SMPParams = append(SMPParams, fmt.Sprintf(",threads=%d", config.SMP.Threads))
		}

		if config.SMP.Sockets > 0 {
			SMPParams = append(SMPParams, fmt.Sprintf(",sockets=%d", config.SMP.Sockets))
		}

		config.qemuParams = append(config.qemuParams, "-smp")
		config.qemuParams = append(config.qemuParams, strings.Join(SMPParams, ""))
	}
}

func (config *Config) appendRTC() {
	if config.RTC.Valid() == false {
		return
	}

	var RTCParams []string

	RTCParams = append(RTCParams, fmt.Sprintf("base=%s", string(config.RTC.Base)))

	if config.RTC.DriftFix != "" {
		RTCParams = append(RTCParams, fmt.Sprintf(",driftfix=%s", config.RTC.DriftFix))
	}

	if config.RTC.Clock != "" {
		RTCParams = append(RTCParams, fmt.Sprintf(",clock=%s", config.RTC.Clock))
	}

	config.qemuParams = append(config.qemuParams, "-rtc")
	config.qemuParams = append(config.qemuParams, strings.Join(RTCParams, ""))
}

func (config *Config) appendGlobalParam() {
	if config.GlobalParam != "" {
		config.qemuParams = append(config.qemuParams, "-global")
		config.qemuParams = append(config.qemuParams, config.GlobalParam)
	}
}

func (config *Config) appendVGA() {
	if config.VGA != "" {
		config.qemuParams = append(config.qemuParams, "-vga")
		config.qemuParams = append(config.qemuParams, config.VGA)
	}
}

func (config *Config) appendKernel() {
	if config.Kernel.Path != "" {
		config.qemuParams = append(config.qemuParams, "-kernel")
		config.qemuParams = append(config.qemuParams, config.Kernel.Path)

		if config.Kernel.Params != "" {
			config.qemuParams = append(config.qemuParams, "-append")
			config.qemuParams = append(config.qemuParams, config.Kernel.Params)
		}
	}
}

func (config *Config) appendKnobs() {
	if config.Knobs.NoUserConfig == true {
		config.qemuParams = append(config.qemuParams, "-no-user-config")
	}

	if config.Knobs.NoDefaults == true {
		config.qemuParams = append(config.qemuParams, "-nodefaults")
	}

	if config.Knobs.NoGraphic == true {
		config.qemuParams = append(config.qemuParams, "-nographic")
	}

	if config.Knobs.Daemonize == true {
		config.qemuParams = append(config.qemuParams, "-daemonize")
	}
}

// LaunchQemu can be used to launch a new qemu instance.
//
// The Config parameter contains a set of qemu parameters and settings.
//
// This function writes its log output via logger parameter.
//
// The function will block until the launched qemu process exits.  "", nil
// will be returned if the launch succeeds.  Otherwise a string containing
// the contents of stderr + a Go error object will be returned.
func LaunchQemu(config Config, logger QMPLog) (string, error) {
	config.appendName()
	config.appendUUID()
	config.appendMachine()
	config.appendCPUModel()
	config.appendQMPSockets()
	config.appendMemory()
	config.appendCPUs()
	config.appendDevices()
	config.appendRTC()
	config.appendGlobalParam()
	config.appendVGA()
	config.appendKnobs()
	config.appendKernel()

	return LaunchCustomQemu(config.Ctx, config.Path, config.qemuParams, config.fds, logger)
}

// LaunchCustomQemu can be used to launch a new qemu instance.
//
// The path parameter is used to pass the qemu executable path.
//
// The ctx parameter is not currently used but has been added so that the
// signature of this function will not need to change when launch cancellation
// is implemented.
//
// config.qemuParams is a slice of options to pass to qemu-system-x86_64 and fds is a
// list of open file descriptors that are to be passed to the spawned qemu
// process.
//
// This function writes its log output via logger parameter.
//
// The function will block until the launched qemu process exits.  "", nil
// will be returned if the launch succeeds.  Otherwise a string containing
// the contents of stderr + a Go error object will be returned.
func LaunchCustomQemu(ctx context.Context, path string, params []string, fds []*os.File, logger QMPLog) (string, error) {
	if logger == nil {
		logger = qmpNullLogger{}
	}

	errStr := ""

	if path == "" {
		path = "qemu-system-x86_64"
	}

	cmd := exec.Command(path, params...)
	if len(fds) > 0 {
		logger.Infof("Adding extra file %v", fds)
		cmd.ExtraFiles = fds
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	logger.Infof("launching qemu with: %v", params)

	err := cmd.Run()
	if err != nil {
		logger.Errorf("Unable to launch qemu: %v", err)
		errStr = stderr.String()
		logger.Errorf("%s", errStr)
	}
	return errStr, err
}
