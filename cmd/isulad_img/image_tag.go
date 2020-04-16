// Copyright (c) Huawei Technologies Co., Ltd. 2019. All rights reserved.
// iSulad-img licensed under the Mulan PSL v1.
// You can use this software according to the terms and conditions of the Mulan PSL v1.
// You may obtain a copy of Mulan PSL v1 at:
//     http://license.coscl.org.cn/MulanPSL
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v1 for more details.
// Description: iSulad image img
// Author: wangfengtu
// Create: 2020-04-15

package main

import (
	"github.com/sirupsen/logrus"
)

func imageTag(gopts *globalOptions, srcName string, destName string) error {
	imageService, err := getImageService(gopts)
	if err != nil {
		return err
	}
	err = imageService.Tag(srcName, destName)
	if err != nil {
		logrus.Debugf("error tagging image %v to %v: %v", srcName, destName, err)
		return err
	}

	return err
}
