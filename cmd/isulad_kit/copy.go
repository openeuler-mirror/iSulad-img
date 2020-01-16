// Copyright (c) Huawei Technologies Co., Ltd. 2019-2020. All rights reserved.
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

// Since some of this code is derived from skopeo, their copyright
// is retained here....
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// The original version of this file can be found at
// https://github.com/containers/skopeo/blob/master/cmd/skopeo/copy.go

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/containers/image/copy"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/manifest"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type copyOptions struct {
	sourceCtx          *types.SystemContext
	destinationCtx     *types.SystemContext
	signBy             string
	removeSignatures   bool
	isSetFormat        bool
	format             string
	isSetAdditionalTag bool
	additionalTag      []string
}

func copyImage(gopts *globalOptions, copts *copyOptions, src string, dest string) error {
	policyContext, err := getPolicyContext(gopts)
	if err != nil {
		return fmt.Errorf("Error loading trust policy: %v", err)
	}
	defer policyContext.Destroy()

	srcRef, err := alltransports.ParseImageName(src)
	if err != nil {
		return fmt.Errorf("Invalid source name %s: %v", src, err)
	}
	destRef, err := alltransports.ParseImageName(dest)
	if err != nil {
		return fmt.Errorf("Invalid destination name %s: %v", dest, err)
	}

	var manifestType string
	if copts.isSetFormat {
		switch copts.format {
		case "oci":
			manifestType = imgspecv1.MediaTypeImageManifest
		case "v2s1":
			manifestType = manifest.DockerV2Schema1SignedMediaType
		case "v2s2":
			manifestType = manifest.DockerV2Schema2MediaType
		default:
			return fmt.Errorf("unknown format %q. Choose on of the supported formats: 'oci', 'v2s1', or 'v2s2'", copts.format)
		}
	}

	if copts.isSetAdditionalTag {
		for _, image := range copts.additionalTag {
			ref, err := reference.ParseNormalizedNamed(image)
			if err != nil {
				return fmt.Errorf("error parsing additional-tag '%s': %v", image, err)
			}
			namedTagged, isNamedTagged := ref.(reference.NamedTagged)
			if !isNamedTagged {
				return fmt.Errorf("additional-tag '%s' must be a tagged reference", image)
			}
			copts.destinationCtx.DockerArchiveAdditionalTags =
				append(copts.destinationCtx.DockerArchiveAdditionalTags, namedTagged)
		}
	}

	_, err = copy.Image(context.Background(), policyContext, destRef, srcRef, &copy.Options{
		RemoveSignatures:      copts.removeSignatures,
		SignBy:                copts.signBy,
		ReportWriter:          os.Stdout,
		SourceCtx:             copts.sourceCtx,
		DestinationCtx:        copts.destinationCtx,
		ForceManifestMIMEType: manifestType,
	})
	return err
}
