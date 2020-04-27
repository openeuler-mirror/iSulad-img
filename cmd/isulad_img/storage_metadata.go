// Copyright (c) Huawei Technologies Co., Ltd. 2020. All rights reserved.
// iSulad-img licensed under the Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//     http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v2 for more details.
// Description: iSulad image kit
// Author: wangfengtu
// Create: 2020-03-20

package main

import "fmt"

func storageMetadata(gopts *globalOptions, nameID string) (metadata map[string]string, name string, err error) {
	imageService, err := getImageService(gopts)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get image service: %v", err)
	}

	runtimeService := getRuntimeService("", imageService)
	if runtimeService == nil {
		return nil, "", fmt.Errorf("failed to get runtime service: %v", err)
	}

	layerID, err := runtimeService.GetContainerLayerID(nameID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get container layer id by %v: %v", nameID, err)
	}

	store, err := getStorageStore(gopts)
	if err != nil {
		return nil, "", err
	}

	driver, err := store.GraphDriver()
	if err != nil {
		return nil, "", err
	}

	metadata, err = driver.Metadata(layerID)
	if err != nil {
		return nil, "", err
	}

	return metadata, driver.String(), err
}
