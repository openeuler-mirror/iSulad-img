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

	"github.com/containers/image/docker"
	"github.com/containers/image/pkg/docker/config"
	"github.com/containers/image/types"
	"github.com/urfave/cli"
)

func loginHandler(c *cli.Context) error {
	if len(c.Args()) != 1 {
		cli.ShowCommandHelp(c, "login")
		return errors.New("Exactly one arguments expected")
	}

	username, password, err := readAuthFromStdin()
	if err != nil {
		return err
	}

	if username == "" || password == "" {
		cli.ShowCommandHelp(c, "login")
		return errors.New("Missing username or password")
	}

	store, err := getStorageStore(true, c)
	if err != nil {
		return err
	}

	ctx, cancel := commandTimeoutContextFromGlobalOptions(c)
	defer cancel()

	svc, err := getImageService(ctx, c, store)
	if err != nil {
		return err
	}

	sys, err := contextFromGlobalOptions(c, "")
	if err != nil {
		return err
	}

	serverAddr := strings.Split(c.Args()[0], "/")[0]

	if secure := svc.IsSecureIndex(serverAddr); !secure {
		sys.DockerInsecureSkipTLSVerify = types.NewOptionalBool(true)
	}

	if err := docker.CheckAuth(ctx, sys, username, password, serverAddr); err != nil {
		return err
	}

	if err := config.SetAuthentication(sys, serverAddr, username, password); err != nil {
		return err
	}

	fmt.Println("Login Succeeded")

	return nil
}

var loginCmd = cli.Command{
	Name:  "login",
	Usage: "isulad_kit login [OPTIONS] SERVER",
	Description: fmt.Sprintf(`

	Log in to a Docker registry
	`),
	ArgsUsage: "",
	Action:    loginHandler,
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
