// Copyright © 2023 Meroxa, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bwlimit

// Byte represents a number of bytes as an int.
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
