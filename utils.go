// Copyright 2025 Dolthub, Inc.
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

package main

/*
#cgo CFLAGS: "-I${SRCDIR}/library"
#include "exports.h"
*/
import "C"
import "unsafe"

// FromDatum converts the given datum to the type.
func FromDatum[T any](d Datum) *T {
	if d == 0 {
		return nil
	}
	return (*T)(unsafe.Pointer(d))
}

// ToDatum converts the given pointer to a Datum.
func ToDatum[T any](val *T) Datum {
	if val == nil {
		return 0
	}
	return Datum(unsafe.Pointer(val))
}

// Malloc allocates the given type within the C heap. These should always be followed up with a Free at some point
// afterward.
func Malloc[T any]() *T {
	var structToDetermineSize T
	return (*T)(C.malloc(C.size_t(unsafe.Sizeof(structToDetermineSize))))
}

// ZeroMemory writes all zeroes to the memory location occupied by the given pointer.
func ZeroMemory[T any](val *T) {
	var structToDetermineSize T
	C.memset(unsafe.Pointer(val), 0, C.size_t(unsafe.Sizeof(structToDetermineSize)))
}

// Free frees the given pointer from C heap. Generally, this is paired with a pointer returned from Malloc.
func Free[T any](val *T) {
	C.free(unsafe.Pointer(val))
}

// FreeDatum frees the given Datum. Care should be exercised as datums may refer to static memory, and attempting to
// free static memory will result in a crash.
func FreeDatum(val Datum) {
	C.free(unsafe.Pointer(val))
}
