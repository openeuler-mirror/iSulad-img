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

func containerFilesystemUsage(gopts *globalOptions, containerName2 string) ([]byte, error) {
	imageService, err := getImageService(gopts)
	if err != nil {
		return nil, err
	}

	storageRuntimeService := getRuntimeService("", imageService)
	if storageRuntimeService == nil {
		return nil, errors.New("Failed to get storageRuntimeService")
	}

	layerID, err := storageRuntimeService.GetContainerLayerID(containerName2)
	if err != nil {
		return nil, fmt.Errorf("failed to get container %s layerid: %v", containerName2, err)
	}

	store, err := getStorageStore(gopts)
	if err != nil {
		return nil, err
	}

	fsUsage, err := getContainerStorageFsInfo(store, layerID)
	if err != nil {
		return nil, err
	}

	resp := &ImageFsInfoResponse{
		ImageFilesystems: []*FilesystemUsage{fsUsage},
	}

	logrus.Debugf("ContainerFsinfoResponse: %+v", resp)

	data, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	return data, err
}
