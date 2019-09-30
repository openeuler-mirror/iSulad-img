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
	"encoding/json"
	"fmt"

	"github.com/containers/image/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	containerImageName, containerName, containerID string
	attempt                                        uint32
)

type prepareResponse struct {
	MountPoint string `json:"mount_point,omitempty"`
}

func validateContainerPrepareConfig(c *cli.Context) error {
	if !c.IsSet("image") {
		return errors.New("no image configured")
	}
	containerImageName = c.String("image")

	if !c.IsSet("name") {
		return errors.New("no container name configured")
	}
	containerName = c.String("name")

	if c.IsSet("id") {
		containerID = c.String("id")
	}

	return nil
}

func containerPrepareHandler(c *cli.Context) error {

	err := validateContainerPrepareConfig(c)
	if err != nil {
		return err
	}

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

	images, err := imageService.ResolveNames(containerImageName)
	if err != nil {
		if err == ErrCannotParseImageID {
			images = append(images, containerImageName)
		} else {
			return err
		}
	}

	// Get imageName and imageRef that are later requested in container status
	var (
		imgResult    *ImageResult
		imgResultErr error
	)
	for _, img := range images {
		imgResult, imgResultErr = imageService.ImageStatus(&types.SystemContext{}, img)
		if imgResultErr == nil {
			break
		}
	}
	if imgResultErr != nil {
		return imgResultErr
	}

	storageOpts, err := getStorageOptions(c)
	if err != nil {
		return err
	}

	containerInfo, err := storageRuntimeService.CreateContainer(&types.SystemContext{},
		containerName, containerName,
		containerImageName, imgResult.ID,
		containerName, containerID,
		"",
		0,
		"",
		nil,
		storageOpts,
		nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err2 := storageRuntimeService.RemoveContainer(containerInfo.ID)
			if err2 != nil {
				logrus.Warnf("Failed to cleanup container directory: %v", err2)
			}
		}
	}()

	container, err := store.Container(containerInfo.ID)
	if err != nil {
		fmt.Errorf("failed to get container %s: %v", containerInfo.ID, err)
	}
	layer, err := store.Layer(container.LayerID)
	if err != nil {
		fmt.Errorf("failed to get container %s layer %s: %v", containerInfo.ID, container.LayerID, err)
	}

	response := &prepareResponse{
		MountPoint: layer.MountPoint,
	}
	data, err := json.Marshal(response)
	if err != nil {
		return err
	}

	fmt.Printf("%s", data)

	return err
}

var containerPrepareCmd = cli.Command{
	Name:  "prepare",
	Usage: "isulad_kit prepare [OPTIONS]",
	Description: fmt.Sprintf(`

	Prepare base rootfs for a container.

	`),
	ArgsUsage: "[OPTIONS]",
	Action:    containerPrepareHandler,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "image",
			Usage: "Name of an image which we use to instantiate container",
		},
		cli.StringFlag{
			Name:  "name",
			Usage: "Name for the container",
		},
		cli.StringFlag{
			Name:  "id",
			Usage: "ID for the container.",
		},
	},
}
