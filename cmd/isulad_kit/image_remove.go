// Copyright (c) Huawei Technologies Co., Ltd. 2019. All rights reserved.
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
	"encoding/json"
	"fmt"

	"github.com/containers/image/types"
	"github.com/sirupsen/logrus"
)

type removeImageResponse struct {
}

func imageRemove(gopts *globalOptions, image string) error {
	imageService, err := getImageService(gopts)
	if err != nil {
		return err
	}

	err = imageService.UnrefImage(&types.SystemContext{}, image)
	if err != nil {
		logrus.Debugf("error deleting image %s: %v", image, err)
		return err
	}

	resp := &removeImageResponse{}
	logrus.Debugf("removeImageResponse: %+v", resp)

	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", data)
	return err
}
