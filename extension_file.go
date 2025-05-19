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

import (
	"cmp"
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// sqlFunctionCapture is a regex to capture the function name as defined in the library. We'll eventually replace this
// and use the nodes from the parser, but this is good enough for the default extensions.
var sqlFunctionCapture = regexp.MustCompile(`(?is)create\s+(?:or\s+replace\s+)?function\s+(.*?)\s*\(.*?\)\s+(?:.*?language c.*?as\s+'.*?'\s*,\s*'(.*?)'.*?;|.*?as\s+'.*?'\s*,\s*'(.*?)'.*?language c.*?;|.*?language c.*?;)`)

// createFunctionStart is a regex to find the beginning of a CREATE FUNCTION statement.
var createFunctionStart = regexp.MustCompile(`(?is)create\s+(?:or\s+replace\s+)?function`)

// ExtensionFiles contains all of the files that are related to or used by an extension.
type ExtensionFiles struct {
	Name            string
	ControlFileName string
	SQLFileNames    []string
	LibraryFileName string
	ControlFileDir  string
	LibraryFileDir  string
}

// LoadExtensions loads information for all extensions that are in the extensions directory of a local Postgres installation.
func LoadExtensions() (map[string]*ExtensionFiles, error) {
	installDirectory, err := PostgresInstallDirectory()
	if err != nil {
		return nil, err
	}
	libDir := fmt.Sprintf("%s/lib", installDirectory)
	extDir := fmt.Sprintf("%s/share/extension", installDirectory)
	dirEntries, err := os.ReadDir(extDir)
	if err != nil {
		return nil, err
	}
	libEntries, err := os.ReadDir(libDir)
	if err != nil {
		return nil, err
	}
	extensionFiles := make(map[string]*ExtensionFiles)
	// Look for the control files first
	for _, dirEntry := range dirEntries {
		fileName := dirEntry.Name()
		if !dirEntry.IsDir() && strings.HasSuffix(fileName, ".control") {
			extensionName := strings.TrimSuffix(fileName, ".control")
			extensionFiles[extensionName] = &ExtensionFiles{
				Name:            extensionName,
				ControlFileName: fileName,
				ControlFileDir:  extDir,
			}
		}
	}
	// Associate the SQL files and libraries
	for _, extFile := range extensionFiles {
		for _, dirEntry := range dirEntries {
			fileName := dirEntry.Name()
			if !dirEntry.IsDir() && strings.HasPrefix(fileName, extFile.Name+"--") && strings.HasSuffix(fileName, ".sql") {
				extFile.SQLFileNames = append(extFile.SQLFileNames, fileName)
			}
		}
		for _, libEntry := range libEntries {
			fileName := libEntry.Name()
			if !libEntry.IsDir() && strings.HasPrefix(fileName, extFile.Name+".") {
				extFile.LibraryFileName = fileName
				extFile.LibraryFileDir = libDir
			}
		}
		slices.SortFunc(extFile.SQLFileNames, func(aStr, bStr string) int {
			a := sqlFileToVersions(extFile.Name, aStr)
			b := sqlFileToVersions(extFile.Name, bStr)
			return cmp.Or(
				cmp.Compare(a[0], b[0]),
				cmp.Compare(a[1], b[1]),
			)
		})
		// Some SQL files are old migration files that won't apply to us, so we can remove them by starting at the first
		// non-migration file.
		for nextLoop := true; nextLoop; {
			nextLoop = false
			for i := 1; i < len(extFile.SQLFileNames); i++ {
				if strings.Count(extFile.SQLFileNames[i], "--") == 1 {
					extFile.SQLFileNames = extFile.SQLFileNames[i:]
					nextLoop = true
					break
				}
			}
		}
	}
	return extensionFiles, nil
}

// LoadControl loads the control file of an extension.
func (extFile *ExtensionFiles) LoadControl() (string, error) {
	data, err := os.ReadFile(fmt.Sprintf("%s/%s", extFile.ControlFileDir, extFile.ControlFileName))
	if err != nil {
		return "", err
	}
	// TODO: create a Control struct and read the contents into that
	return string(data), nil
}

// LoadSQLFiles loads the contents of the SQL files used by the extension. These will be in the order that they need to
// be executed.
func (extFile *ExtensionFiles) LoadSQLFiles() ([]string, error) {
	sqlFiles := make([]string, len(extFile.SQLFileNames))
	for i, sqlFileName := range extFile.SQLFileNames {
		data, err := os.ReadFile(fmt.Sprintf("%s/%s", extFile.ControlFileDir, sqlFileName))
		if err != nil {
			return nil, err
		}
		sqlFiles[i] = string(data)
	}
	return sqlFiles, nil
}

// LoadSQLFunctionNames loads all of the library function names that are used by the extension.
func (extFile *ExtensionFiles) LoadSQLFunctionNames() ([]string, error) {
	funcNames := make(map[string]struct{})
	for _, sqlFileName := range extFile.SQLFileNames {
		data, err := os.ReadFile(fmt.Sprintf("%s/%s", extFile.ControlFileDir, sqlFileName))
		if err != nil {
			return nil, err
		}
		fileRemaining := string(data)
		for {
			// We want to advance the file to the start of the next CREATE FUNCTION if one is present
			startIdx := createFunctionStart.FindStringIndex(fileRemaining)
			if startIdx == nil {
				break
			}
			fileRemaining = fileRemaining[startIdx[0]:]
			// We capture the ending semicolon so the regex doesn't match beyond the function definition's boundaries.
			endIdx := strings.IndexRune(fileRemaining, ';')
			if endIdx == -1 {
				break
			}
			matches := sqlFunctionCapture.FindStringSubmatch(fileRemaining[:endIdx+1])
			switch len(matches) {
			case 0:
				break
			case 4:
				if len(matches[2]) > 0 {
					funcNames[matches[2]] = struct{}{}
				} else if len(matches[3]) > 0 {
					funcNames[matches[3]] = struct{}{}
				} else {
					funcNames[matches[1]] = struct{}{}
				}
			default:
				return nil, fmt.Errorf("invalid CREATE FUNCTION string: %s", string(data))
			}
			// We nudge it forward to guarantee that our next CREATE FUNCTION search will grab the next one
			fileRemaining = fileRemaining[6:]
		}
	}
	sortedFuncNames := slices.Sorted(maps.Keys(funcNames))
	return sortedFuncNames, nil
}

// LoadLibrary loads the extension as a library.
func (extFile *ExtensionFiles) LoadLibrary() (*Library, error) {
	if len(extFile.LibraryFileName) == 0 {
		return nil, fmt.Errorf("extension `%s` does not reference a library", extFile.Name)
	}
	funcNames, err := extFile.LoadSQLFunctionNames()
	if err != nil {
		return nil, err
	}
	return LoadLibrary(fmt.Sprintf("%s/%s", extFile.LibraryFileDir, extFile.LibraryFileName), funcNames)
}

// sqlFileToVersions decodes the version information within the SQL file name.
func sqlFileToVersions(name string, sqlFileName string) [2]uint16 {
	if !strings.HasSuffix(sqlFileName, ".sql") {
		return [2]uint16{}
	}
	versionSubsection := strings.TrimSuffix(sqlFileName[len(name)+2: /* We add 2 to account for the -- */], ".sql")
	var from, to string
	if dashIdx := strings.Index(versionSubsection, "--"); dashIdx == -1 {
		from = versionSubsection
		to = versionSubsection
	} else {
		from = versionSubsection[:dashIdx]
		to = versionSubsection[dashIdx+2:]
	}
	fromSplit := strings.Index(from, ".")
	toSplit := strings.Index(to, ".")
	if fromSplit == -1 || toSplit == -1 {
		return [2]uint16{}
	}
	fromMajor, err := strconv.Atoi(from[:fromSplit])
	if err != nil {
		return [2]uint16{}
	}
	fromMinor, err := strconv.Atoi(from[fromSplit+1:])
	if err != nil {
		return [2]uint16{}
	}
	toMajor, err := strconv.Atoi(to[:toSplit])
	if err != nil {
		return [2]uint16{}
	}
	toMinor, err := strconv.Atoi(to[toSplit+1:])
	if err != nil {
		return [2]uint16{}
	}
	return [2]uint16{(uint16(fromMajor) << 8) + uint16(fromMinor), (uint16(toMajor) << 8) + uint16(toMinor)}
}
