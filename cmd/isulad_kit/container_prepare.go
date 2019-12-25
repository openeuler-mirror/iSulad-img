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
	"github.com/containers/image/types"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func containerPrepare(gopts *globalOptions, storageOpts map[string]string,
	containerImageName string, containerName string, containerID string) (string, *v1.Image, error) {
	var sopts map[string]string

	imageService, err := getImageService(gopts)
	if err != nil {
		return "", nil, err
	}

	storageRuntimeService := getRuntimeService("", imageService)
	if storageRuntimeService == nil {
		return "", nil, errors.New("Failed to get storageRuntimeService")
	}

	// Get imageName and imageRef that are later requested in container status
	var (
		imgBasicSpec    *ImageBasicSpec
		imgBasicSpecErr error
	)
	imgBasicSpec, imgBasicSpecErr = imageService.GetOneImage(&types.SystemContext{}, containerImageName)
	if imgBasicSpecErr != nil {
		return "", nil, imgBasicSpecErr
	}

	if storageOpts != nil {
		sopts = storageOpts
	} else {
		sopts = gopts.storageOpts
	}

	containerInfo, err := storageRuntimeService.CreateContainer(&types.SystemContext{},
		containerImageName, imgBasicSpec.ID,
		containerName, containerID,
		sopts)
	if err != nil {
		return "", nil, err
	}
	defer func() {
		if err != nil {
			err2 := storageRuntimeService.RemoveContainer(containerInfo.ID)
			if err2 != nil {
				logrus.Warnf("Failed to cleanup container directory: %v", err2)
			}
		}
	}()

	return containerInfo.MountPoint, containerInfo.Config, err
}
