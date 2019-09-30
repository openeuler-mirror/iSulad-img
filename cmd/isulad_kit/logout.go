// Copyright (c) Huawei Technologies Co., Ltd. 2019-2019. All rights reserved.
// iSulad-kit licensed under the Mulan PSL v1.
// You can use this software according to the terms and conditions of the Mulan PSL v1.
// You may obtain a copy of Mulan PSL v1 at:
//     http://license.coscl.org.cn/MulanPSL
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v1 for more details.
// Description: iSulad login kit
// Author: wangfengtu
// Create: 2019-06-17

package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/containers/image/pkg/docker/config"
	"github.com/urfave/cli"
)

func logoutHandler(c *cli.Context) error {
	if len(c.Args()) != 1 {
		cli.ShowCommandHelp(c, "logout")
		return errors.New("Exactly one arguments expected")
	}

	sys, err := contextFromGlobalOptions(c, "")
	if err != nil {
		return err
	}

	serverAddr := strings.Split(c.Args()[0], "/")[0]
	err = config.RemoveAuthentication(sys, serverAddr)
	if err == nil {
		fmt.Printf("Removing login credentials for %v\n", serverAddr)
		return nil
	}
	if strings.Contains(err.Error(), "not logged in") {
		fmt.Printf("Not logged in to %v\n", serverAddr)
		return nil
	}

	return err
}

var logoutCmd = cli.Command{
	Name:  "logout",
	Usage: "isulad_kit logout SERVER",
	Description: fmt.Sprintf(`

	Log out from a Docker registry
	`),
	ArgsUsage: "",
	Action:    logoutHandler,
	Flags: []cli.Flag{
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
