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
// Create: 2020-05-25

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/containers/image/copy"
	"github.com/containers/image/storage"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
)

func importImage(gopts *globalOptions, input string, destTag string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("Missing input tarball name")
	}

	imageService, err := getImageService(gopts)
	if err != nil {
		return "", fmt.Errorf("get image service failed: %v", err)
	}

	policyContext, err := getPolicyContext(gopts)
	if err != nil {
		return "", fmt.Errorf("Error loading trust policy: %v", err)
	}
	defer policyContext.Destroy()

	store, err := getStorageStore(gopts)
	if err != nil {
		return "", err
	}

	// Make sure file exist, so we can make sure the following formated input always valid
	_, err = os.Stat(input)
	if err != nil {
		return "", err
	}

	// Format: tarball:/path/rootfs.tar
	formatedInput := "tarball:" + input
	srcRef, err := alltransports.ParseImageName(formatedInput)
	if err != nil {
		return "", fmt.Errorf("Invalid input name %s: %v", formatedInput, err)
	}

	destRef, err := storage.Transport.ParseStoreReference(store, destTag)
	if err != nil {
		return "", fmt.Errorf("Invalid tag %s: %v", destTag, err)
	}

	_, err = copy.Image(context.Background(), policyContext, destRef, srcRef, &copy.Options{
		ReportWriter: os.Stdout,
	})
	if err != nil {
		return "", fmt.Errorf("Import rootfs %v failed: %v", input, err)
	}

	status, err := imageService.GetOneImage(&types.SystemContext{}, destTag)
	if err != nil {
		return "", err
	}

	fmt.Println(status.ID)

	return status.ID, err
}
