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
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func storageStatusHandler(c *cli.Context) error {
	if len(c.Args()) > 0 {
		cli.ShowCommandHelp(c, "driver_status")
		return errors.New("No arguments expected")
	}

	logrus.Debugf("Driver Status Requested")

	store, err := getStorageStore(false, c)
	if err != nil {
		return err
	}

	driver, err := store.GraphDriver()
	if err != nil {
		return err
	}

	status := driver.Status()
	if err != nil {
		return err
	}

	for _, value := range status {
		fmt.Printf("%s: %s\n", value[0], value[1])
	}

	return err
}

var storageStatusCmd = cli.Command{
	Name:  "storage_status",
	Usage: "isulad_kit storage_status",
	Description: fmt.Sprintf(`

	Display detailed information of the image storage driver.

	`),
	ArgsUsage: " ",
	Action:    storageStatusHandler,
	Flags: []cli.Flag{},
}
