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

package pg_extension

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
)

func TestSmoke(t *testing.T) {
	extensionFiles, err := LoadExtensions()
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	lib, err := extensionFiles["uuid-ossp"].LoadLibrary()
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	defer func() {
		_ = lib.internal.Close()
	}()
	fmt.Printf("Pg_magic_func:\n  version=%d  maxArgs=%d  nameDataLen=%d\n",
		lib.Magic.Version, lib.Magic.FuncMaxArgs, lib.Magic.NameDataLen)
	datum, isNotNull := CallFmgrFunction(lib.Funcs["uuid_generate_v4"].Ptr)
	if isNotNull {
		val := FromDatumGoBytes(datum, 16)
		FreeDatum(datum)
		uuidVal, _ := uuid.FromBytes(val)
		fmt.Printf("uuid_generate_v4:\n  %v\n", uuidVal.String())
	} else {
		fmt.Printf("uuid_generate_v4:\n  null\n")
	}
}
