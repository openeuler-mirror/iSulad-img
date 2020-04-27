// Copyright (c) Huawei Technologies Co., Ltd. 2019. All rights reserved.
// iSulad-img licensed under the Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//     http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v2 for more details.
// Description: iSulad image kit
// Author: lifeng
// Create: 2019-05-06

package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/containers/image/copy"
	"github.com/containers/image/types"
	"github.com/sirupsen/logrus"
)

type pullOptions struct {
	username  string
	password  string
	certDir   string
	tlsVerify bool
}

func decodeAuth(s string) (string, string, error) {
	decodeStr, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", "", err
	}
	parts := strings.SplitN(string(decodeStr), ":", 2)
	// should be username and password
	if len(parts) != 2 {
		return "", "", nil
	}
	username := parts[0]
	password := strings.Trim(parts[1], "\x00")
	return username, password, nil
}

func imagePull(gopts *globalOptions, popts *pullOptions, image string) (string, error) {
	imageService, err := getImageService(gopts)
	if err != nil {
		return "", err
	}

	// print the download report to stderr for debug
	options := &copy.Options{
		ReportWriter: os.Stderr,
	}

	options.SourceCtx = &types.SystemContext{
		DockerCertPath:              popts.certDir,
		DockerInsecureSkipTLSVerify: types.NewOptionalBool(!popts.tlsVerify),
		AuthFilePath:                defaultAuthFilePath(),
	}

	// Specifying a username indicates the user intends to send authentication to the registry.
	if popts.username != "" {
		options.SourceCtx.DockerAuthConfig = &types.DockerAuthConfig{
			Username: popts.username,
			Password: popts.password,
		}
	}

	var (
		pulled string
	)
	images, err := imageService.ParseImageNames(image)
	if err != nil {
		return "", err
	}

	dstImage := image
	for _, srcImage := range images {
		var tmpImg types.Image
		tmpImg, err = imageService.InitImage(srcImage, options)
		if err != nil {
			logrus.Debugf("error preparing image %s: %v", srcImage.name, err)
			continue
		}

		var storedImage *ImageBasicSpec
		storedImage, err = imageService.GetOneImage(&types.SystemContext{}, dstImage)
		if err == nil {
			tmpImgConfigDigest := tmpImg.ConfigInfo().Digest
			if tmpImgConfigDigest.String() == "" {
				logrus.Debugf("image config digest is empty, re-pulling image")
			} else if tmpImgConfigDigest.String() == storedImage.ConfigDigest.String() {
				logrus.Debugf("image %s already in store, skipping pull", dstImage)
				pulled = dstImage
				break
			}
			logrus.Debugf("image in store has different ID, re-pulling %s", dstImage)
		}

		_, err = imageService.PullImage(&types.SystemContext{}, srcImage, dstImage, options)
		if err != nil {
			logrus.Debugf("error pulling image %s: %v", srcImage.name, err)
			continue
		}
		pulled = dstImage
		break
	}
	if pulled == "" && err != nil {
		return "", err
	}
	status, err := imageService.GetOneImage(&types.SystemContext{}, pulled)
	if err != nil {
		return "", err
	}

	fmt.Print(status.ID)
	return status.ID, nil
}
