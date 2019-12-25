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
	"path"
	"time"

	"github.com/containers/storage"
)

// FilesystemIdentifier uniquely identify the filesystem.
type FilesystemIdentifier struct {
	// Mountpoint of a filesystem.
	Mountpoint string `json:"mountpoint,omitempty"`
}

// UInt64Value is the wrapper of uint64.
type UInt64Value struct {
	// The value.
	Value uint64 `json:"value,omitempty"`
}

//FilesystemUsage provides the filesystem usage information.
type FilesystemUsage struct {
	// Timestamp in nanoseconds at which the information were collected. Must be > 0.
	Timestamp int64 `json:"timestamp,omitempty"`
	// The unique identifier of the filesystem.
	FsID *FilesystemIdentifier `json:"fs_id,omitempty"`
	// UsedBytes represents the bytes used for images on the filesystem.
	// This may differ from the total bytes used on the filesystem and may not
	// equal CapacityBytes - AvailableBytes.
	UsedBytes *UInt64Value `json:"used_bytes,omitempty"`
	// InodesUsed represents the inodes used by the images.
	// This may not equal InodesCapacity - InodesAvailable because the underlying
	// filesystem may also be used for purposes other than storing images.
	InodesUsed *UInt64Value `json:"inodes_used,omitempty"`
}

// ImageFsInfoResponse provides filesystem usage information.
type ImageFsInfoResponse struct {
	// Information of image filesystem(s).
	ImageFilesystems []*FilesystemUsage `json:"image_filesystems,omitempty"`
}

func getStorageFsInfo(store storage.Store) (*FilesystemUsage, error) {
	rootPath := store.GraphRoot()
	storageDriver := store.GraphDriverName()
	imagesPath := path.Join(rootPath, storageDriver+"-images")

	bytesUsed, inodesUsed, err := GetDiskUsageStats(imagesPath)
	if err != nil {
		return nil, err
	}

	usage := FilesystemUsage{
		Timestamp:  time.Now().UnixNano(),
		FsID:       &FilesystemIdentifier{Mountpoint: imagesPath},
		UsedBytes:  &UInt64Value{Value: bytesUsed},
		InodesUsed: &UInt64Value{Value: inodesUsed},
	}

	return &usage, nil
}

func imageFsinfo(gopts *globalOptions) ([]*FilesystemUsage, error) {
	store, err := getStorageStore(gopts)
	if err != nil {
		return nil, err
	}

	fsUsage, err := getStorageFsInfo(store)
	if err != nil {
		return nil, err
	}

	return []*FilesystemUsage{fsUsage}, err
}
