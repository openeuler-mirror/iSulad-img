// Copyright (c) Huawei Technologies Co., Ltd. 2019. All rights reserved.
// iSulad-img licensed under the Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//     http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v2 for more details.
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
		// Ignore invalid files
		if err != nil {
			return nil
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
