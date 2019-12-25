// Copyright (c) Huawei Technologies Co., Ltd. 2019. All rights reserved.
// iSulad-kit licensed under the Mulan PSL v1.
// You can use this software according to the terms and conditions of the Mulan PSL v1.
// You may obtain a copy of Mulan PSL v1 at:
//     http://license.coscl.org.cn/MulanPSL
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v1 for more details.
// Description: iSulad image kit
// Author: lifeng
// Create: 2019-05-06

package main

import (
	"os"
	"path/filepath"
)

// GetDiskUsageStats accepts a path to a directory or file
// and returns the number of bytes and inodes used by the path
func GetDiskUsageStats(path string) (uint64, uint64, error) {
	var dirSize, inodeCount uint64

	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		fileStat, error := os.Lstat(path)
		if error != nil {
			if fileStat.Mode()&os.ModeSymlink != 0 {
				// Is a symlink; no error should be returned
				return nil
			}
			return error
		}

		dirSize += uint64(info.Size())
		inodeCount++

		return nil
	})

	if err != nil {
		return 0, 0, err
	}

	return dirSize, inodeCount, err
}
