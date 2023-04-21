package bwlimit

type Byte int

// Base-2 byte units.
const (
	Kibibyte Byte = 1024
	KiB           = Kibibyte
	Mebibyte      = Kibibyte * 1024
	MiB           = Mebibyte
	Gibibyte      = Mebibyte * 1024
	GiB           = Gibibyte
)

// SI base-10 byte units.
const (
	Kilobyte Byte = 1000
	KB            = Kilobyte
	Megabyte      = Kilobyte * 1000
	MB            = Megabyte
	Gigabyte      = Megabyte * 1000
	GB            = Gigabyte
)
