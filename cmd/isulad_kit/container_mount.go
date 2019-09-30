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
	"fmt"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

func mountHandler(c *cli.Context) error {

	if len(c.Args()) != 1 {
		cli.ShowCommandHelp(c, "mount")
		return errors.New("Exactly one arguments expected")
	}

	idOrName := c.Args()[0]
	mountPoint, err := getMountPoint(c, idOrName)
	if err != nil {
		return fmt.Errorf("failed to mount container %s: %v", idOrName, err)
	}
	fmt.Print(mountPoint)
	return err
}

var mountCmd = cli.Command{
	Name:  "mount",
	Usage: "isulad_kit mount [ID|NAME]",
	Description: fmt.Sprintf(`

	Mount a container's filesystem, and returns the location of its root filesystem.

	`),
	ArgsUsage: "[ID|NAME]",
	Action:    mountHandler,
	Flags:     []cli.Flag{},
}
