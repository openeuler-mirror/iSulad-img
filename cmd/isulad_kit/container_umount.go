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

func uMountHandler(c *cli.Context) error {

	if len(c.Args()) != 1 {
		cli.ShowCommandHelp(c, "umount")
		return errors.New("Exactly one arguments expected")
	}

	idOrName := c.Args()[0]
	store, err := getStorageStore(true, c)
	if err != nil {
		return err
	}

	ctx, cancel := commandTimeoutContextFromGlobalOptions(c)
	defer cancel()

	imageService, err := getImageService(ctx, c, store)
	if err != nil {
		return err
	}

	storageRuntimeService := getRuntimeService(ctx, "", imageService)
	if storageRuntimeService == nil {
		return errors.New("Failed to get storageRuntimeService")
	}

	err = storageRuntimeService.UmountContainer(idOrName)
	if err != nil {
		return fmt.Errorf("failed to unmount container %s: %v", idOrName, err)
	}
	return err
}

var uMountCmd = cli.Command{
	Name:  "umount",
	Usage: "isulad_kit umount [ID|NAME]",
	Description: fmt.Sprintf(`

	Umount a mounted container's filesystem.

	`),
	ArgsUsage: "[ID|NAME]",
	Action:    uMountHandler,
	Flags:     []cli.Flag{},
}
