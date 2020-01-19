// Copyright (c) Huawei Technologies Co., Ltd. 2019. All rights reserved.
// iSulad-img licensed under the Mulan PSL v1.
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
	"fmt"
	"strings"

	"github.com/containers/image/pkg/docker/config"
	"github.com/containers/image/types"
)

func logoutRegistry(sys *types.SystemContext, serverAddr string) error {
	err := config.RemoveAuthentication(sys, serverAddr)
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
