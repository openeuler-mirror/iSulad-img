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
	"path"
	"time"

	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	containerName2 string
)

func getContainerStorageFsInfo(store storage.Store, containerpath string) (*FilesystemUsage, error) {
	rootPath := store.GraphRoot()
	storageDriver := store.GraphDriverName()
	containerPath := path.Join(rootPath, storageDriver, containerpath, "diff")

	bytesUsed, inodesUsed, err := GetDiskUsageStats(containerPath)
	if err != nil {
		return nil, err
	}

	usage := FilesystemUsage{
		Timestamp:  time.Now().UnixNano(),
		FsID:       &FilesystemIdentifier{Mountpoint: containerPath},
		UsedBytes:  &UInt64Value{Value: bytesUsed},
		InodesUsed: &UInt64Value{Value: inodesUsed},
	}

	return &usage, nil
}

func containerFilesystemUsageHandler(c *cli.Context) error {

	if len(c.Args()) != 1 {
		cli.ShowCommandHelp(c, "filesystemusage")
		return errors.New("Exactly one arguments expected")
	}

	containerName2 := c.Args()[0]
	logrus.Debugf("filesystem usage Request: %+v", containerName2)

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

	layerID, err := storageRuntimeService.GetContainerLayerID(containerName2)
	if err != nil {
		return fmt.Errorf("failed to get container %s layerid: %v", containerName2, err)
	}

	fsUsage, err := getContainerStorageFsInfo(store, layerID)

	if err != nil {
		return err
	}

	resp := &ImageFsInfoResponse{
		ImageFilesystems: []*FilesystemUsage{fsUsage},
	}

	logrus.Debugf("ContainerFsinfoResponse: %+v", resp)

	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", data)
	return err
}

var containerFilesystemUsageCmd = cli.Command{
	Name:  "filesystemusage",
	Usage: "isulad_kit containerfsinfo [OPTIONS]",
	Description: fmt.Sprintf(`

	Get container filesystem usage.

	`),
	ArgsUsage: "[ID|NAME]",
	Action:    containerFilesystemUsageHandler,
	Flags:     []cli.Flag{},
}
