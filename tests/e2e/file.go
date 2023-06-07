// Copyright © 2023 Cisco Systems, Inc. and/or its affiliates
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

package e2e

import (
	"io/fs"
	"os"

	"emperror.dev/errors"
)

// createTempFileFromBytes creates a temporary file in the file system with the
// specified content in the provided temporary directory and temporary file name
// pattern using the given file mode override and returns the file path or
// alternatively an error.
func createTempFileFromBytes(
	content []byte,
	tempDirectoryOverride string,
	tempFileNamePatternOverride string,
	fileModeOverride fs.FileMode,
) (string, error) {
	if fileModeOverride == 0 {
		fileModeOverride = 0o777
	}

	tempFile, err := os.CreateTemp(tempDirectoryOverride, tempFileNamePatternOverride)
	if err != nil {
		return "", errors.WrapIfWithDetails(
			err,
			"creating temporary file failed",
			"content", string(content),
			"tempDirectoryOverride", tempDirectoryOverride,
			"tempFileNamePatternOverride", tempFileNamePatternOverride,
		)
	}

	err = os.WriteFile(tempFile.Name(), content, fileModeOverride)
	if err != nil {
		return "", errors.WrapIfWithDetails(
			err,
			"writing content to temporary file failed",
			"fileName", tempFile.Name(),
			"content", string(content),
			"fileModeOverride", fileModeOverride,
		)
	}

	err = tempFile.Close()
	if err != nil {
		return "", errors.WrapIfWithDetails(err, "closing temporary file failed", "tempPath", tempFile.Name())
	}

	return tempFile.Name(), nil
}
