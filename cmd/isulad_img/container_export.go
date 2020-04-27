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
	"fmt"
	"io"
	"os"

	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/chrootarchive"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/ioutils"
)

type exportOptions struct {
	file        string
	uid         int
	gid         int
	isSetOffset bool
	offset      int
}

func exportRootfs(gopts *globalOptions, eopts *exportOptions, idOrName string) error {
	output, err := os.OpenFile(eopts.file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("Error creating file %s: %v", eopts.file, err)
	}
	defer output.Close()

	mountPoint, err := getMountPoint(gopts, idOrName)
	if err != nil {
		return fmt.Errorf("failed to mount container %s: %v", idOrName, err)
	}
	defer putMountPoint(gopts, idOrName, false)

	/* offset default to be 65535 if not set*/
	uid := eopts.uid
	gid := eopts.gid
	offset := 65535
	if eopts.isSetOffset {
		offset = eopts.offset
	}

	if uid < 0 || gid < 0 {
		return fmt.Errorf("uid/gid must be greater or equal than 0, got uid(%v) gid(%v)", uid, gid)
	}

	if offset <= 0 {
		return fmt.Errorf("offset must be greater than 0, got %v", offset)
	}

	archive, err := chrootarchive.Tar(mountPoint, &archive.TarOptions{
		Compression: archive.Uncompressed,
		UIDMaps: []idtools.IDMap{
			{
				ContainerID: 0,
				HostID:      uid,
				Size:        offset,
			},
		},
		GIDMaps: []idtools.IDMap{
			{
				ContainerID: 0,
				HostID:      gid,
				Size:        offset,
			},
		},
	}, mountPoint)
	if err != nil {
		return err
	}

	arch := ioutils.NewReadCloserWrapper(archive, func() error {
		err := archive.Close()
		return err
	})
	defer arch.Close()

	if _, err := io.Copy(output, arch); err != nil {
		return fmt.Errorf("Error exporting container %s: %v", idOrName, err)
	}

	return nil
}
