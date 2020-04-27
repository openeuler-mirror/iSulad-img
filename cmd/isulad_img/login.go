// Copyright (c) Huawei Technologies Co., Ltd. 2019. All rights reserved.
// iSulad-img licensed under the Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//     http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v2 for more details.
// Description: iSulad login kit
// Author: wangfengtu
// Create: 2019-06-17

package main

import (
	"context"
	"strings"

	"github.com/containers/image/docker"
	"github.com/containers/image/pkg/docker/config"
	"github.com/containers/image/types"
)

func loginRegistry(gopts *globalOptions, sys *types.SystemContext, username string, password string, server string) error {
	svc, err := getImageService(gopts)
	if err != nil {
		return err
	}

	serverAddr := strings.Split(server, "/")[0]
	if secure := svc.IsSecureIndex(serverAddr); !secure {
		sys.DockerInsecureSkipTLSVerify = types.NewOptionalBool(true)
	}

	if err := docker.CheckAuth(context.Background(), sys, username, password, serverAddr); err != nil {
		return err
	}

	if err := config.SetAuthentication(sys, serverAddr, username, password); err != nil {
		return err
	}

	return nil
}
