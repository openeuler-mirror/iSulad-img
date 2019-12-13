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
	"errors"
	"fmt"
	"os"
	"strings"

	"encoding/base64"
	"github.com/containers/image/copy"
	"github.com/containers/image/types"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

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

func pullHandler(c *cli.Context) error {
	if len(c.Args()) != 1 {
		cli.ShowCommandHelp(c, "pull")
		return errors.New("Exactly one arguments expected")
	}

	image := c.Args()[0]
	logrus.Debugf("Pull Image Request: %+v", image)

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

	username, password, err := readAuthFromStdin()
	if err != nil {
		return err
	}

	if c.IsSet("creds") {
		username, password, err = parseCreds(c.String("creds"))
		if err != nil {
			return err
		}
	}
	if c.IsSet("auth") {
		username, password, err = decodeAuth(c.String("auth"))
		if err != nil {
			return fmt.Errorf("error decoding authentication for image %s: %v", image, err)
		}
	}

	// print the download report to stderr for debug
	options := &copy.Options{
		ReportWriter: os.Stderr,
	}

	options.SourceCtx = &types.SystemContext{
		DockerCertPath:              c.String("cert-dir"),
		DockerInsecureSkipTLSVerify: types.NewOptionalBool(!c.BoolT("tls-verify")),
		UseDecryptedKey:             useDecryptedKey(c, ""),
		AuthFilePath:                defaultAuthFilePath(),
	}

	// Specifying a username indicates the user intends to send authentication to the registry.
	if username != "" {
		options.SourceCtx.DockerAuthConfig = &types.DockerAuthConfig{
			Username: username,
			Password: password,
		}
	}

	var (
		images []string
		pulled string
	)
	images, err = imageService.ParseImageNames(image)
	if err != nil {
		return err
	}

	for _, img := range images {
		var tmpImg types.Image
		tmpImg, err = imageService.InitImage(img, options)
		if err != nil {
			logrus.Debugf("error preparing image %s: %v", img, err)
			continue
		}

		var storedImage *ImageBasicSpec
		storedImage, err = imageService.GetOneImage(&types.SystemContext{}, img)
		if err == nil {
			tmpImgConfigDigest := tmpImg.ConfigInfo().Digest
			if tmpImgConfigDigest.String() == "" {
				logrus.Debugf("image config digest is empty, re-pulling image")
			} else if tmpImgConfigDigest.String() == storedImage.ConfigDigest.String() {
				logrus.Debugf("image %s already in store, skipping pull", img)
				pulled = img
				break
			}
			logrus.Debugf("image in store has different ID, re-pulling %s", img)
		}

		_, err = imageService.PullImage(&types.SystemContext{}, img, options)
		if err != nil {
			logrus.Debugf("error pulling image %s: %v", img, err)
			continue
		}
		pulled = img
		break
	}
	if pulled == "" && err != nil {
		return err
	}
	status, err := imageService.GetOneImage(&types.SystemContext{}, pulled)
	if err != nil {
		return err
	}
	imageRef := status.ID
	if len(status.RepoDigests) > 0 {
		imageRef = status.RepoDigests[0]
	}
	fmt.Print(imageRef)
	return nil
}

var pullCmd = cli.Command{
	Name:  "pull",
	Usage: "isulad_kit pull [OPTIONS] NAME[:TAG|@DIGEST]",
	Description: fmt.Sprintf(`

	Pull an image or a repository from a registry.
	`),
	ArgsUsage: "NAME[:TAG|@DIGEST]",
	Action:    pullHandler,
	// FIXME: Do we need to namespace the GPG aspect?
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "creds",
			Value: "",
			Usage: "Use `USERNAME[:PASSWORD]` for accessing the source registry",
		},
		cli.StringFlag{
			Name:  "auth",
			Value: "",
			Usage: "Use `auth config` for accessing the source registry",
		},
		cli.StringFlag{
			Name:  "cert-dir",
			Value: "",
			Usage: "use certificates at `PATH` (*.crt, *.cert, *.key) to connect to the source registry or daemon",
		},
		cli.BoolTFlag{
			Name:  "tls-verify",
			Usage: "require HTTPS and verify certificates when talking to the container source registry or daemon (defaults to true)",
		},
		cli.BoolTFlag{
			Name:  "use-decrypted-key",
			Usage: "Use decrypted private key by default (defaults to true)",
		},
	},
}
