// Copyright (c) Huawei Technologies Co., Ltd. 2019-2019. All rights reserved.
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
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/ioutils"
	"github.com/urfave/cli"
)

func exportHandler(c *cli.Context) error {
	if len(c.Args()) != 1 {
		cli.ShowCommandHelp(c, "export")
		return errors.New("Exactly one arguments expected")
	}

	file := c.String("output")
	if file == "" {
		return errors.New("Please use --output parameter to specify output file")
	}

	output, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("Error creating file %s: %v", file, err)
	}
	defer output.Close()

	idOrName := c.Args()[0]
	mountPoint, err := getMountPoint(c, idOrName)
	if err != nil {
		return fmt.Errorf("failed to mount container %s: %v", idOrName, err)
	}

	/* offset default to be 65535 if not set*/
	uid := c.Int("uid")
	gid := c.Int("gid")
	offset := 65535
	if c.IsSet("offset") {
		offset = c.Int("offset")
	}

	if uid < 0 || gid < 0 {
		return fmt.Errorf("uid/gid must be greater or equal than 0, got uid(%v) gid(%v)", uid, gid)
	}

	if offset <= 0 {
		return fmt.Errorf("offset must be greater than 0, got %v", offset)
	}

	archive, err := archive.TarWithOptions(mountPoint, &archive.TarOptions{
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
	},
	)
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

var exportCmd = cli.Command{
	Name:  "export",
	Usage: "isulad_kit export [OPTIONS] [ID|NAME]",
	Description: fmt.Sprintf(`

	Export a container's filesystem as a tar archive

	`),
	ArgsUsage: "[ID|NAME]",
	Action:    exportHandler,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "output",
			Usage: "Write to a file",
		},
		cli.IntFlag{
			Name:  "uid",
			Usage: "Specify UID",
		},
		cli.IntFlag{
			Name:  "gid",
			Usage: "Specify GID",
		},
		cli.IntFlag{
			Name:  "offset",
			Usage: "Specify OFFSET",
		},
	},
}
