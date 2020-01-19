// Copyright (c) Huawei Technologies Co., Ltd. 2019. All rights reserved.
// iSulad-img licensed under the Mulan PSL v1.
// You can use this software according to the terms and conditions of the Mulan PSL v1.
// You may obtain a copy of Mulan PSL v1 at:
//     http://license.coscl.org.cn/MulanPSL
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v1 for more details.
// Description: iSulad image kit
// Author: wangfengtu
// Create: 2019-07-12

package main

import (
	"fmt"

	"github.com/urfave/cli"
)

const (
	defaultGrpcAddress = "unix:///var/run/isula_image.sock"
	defaultInfoFile    = "/var/run/isula_image.info"
)

func daemonHandler(c *cli.Context) error {
	var address string
	if c.IsSet("host") {
		address = c.String("host")
	} else {
		address = defaultGrpcAddress
	}

	gopts, err := getGlobalOptions(c)
	if err != nil {
		return err
	}
	gopts.Daemon = true

	// Only one instance is allowed
	if err := newInfoFile(defaultInfoFile, address); err != nil {
		return err
	}
	defer delInfoFile(defaultInfoFile)

	// Init global image service and runtime service
	isrv, err := getImageService(gopts)
	if err != nil {
		return err
	}
	getRuntimeService("", isrv)

	return startGrpcService(daemonOptions{
		Address: address,
		gopts:   gopts,
	})
}

var daemonCmd = cli.Command{
	Name:  "daemon",
	Usage: "run as a daemon",
	Description: fmt.Sprintf(`

	Run as a daemon
	`),
	ArgsUsage: "",
	Action:    daemonHandler,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "H,host",
			Value: "",
			Usage: "Daemon socket(s) to connect to",
		},
		cli.BoolTFlag{
			Name:  "tls-verify",
			Usage: "require HTTPS and verify certificates when talking to the container source registry or daemon (defaults to true)",
		},
		cli.BoolTFlag{
			Name:  "use-decrypted-key",
			Usage: "Use decrypted private key by default (defaults to true)",
		},
	},
}
