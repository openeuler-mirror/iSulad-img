// Copyright (c) Huawei Technologies Co., Ltd. 2019-2020. All rights reserved.
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

// Since some of this code is derived from cri-o, their copyright
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
//
// The original version of this file can be found at
// https://github.com/cri-o/cri-o/blob/master/internal/pkg/storage/image.go

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path"
	"strings"
	"time"

	"github.com/containers/image/copy"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/manifest"
	"github.com/containers/image/signature"
	imstorage "github.com/containers/image/storage"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	digest "github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

var (
	// minIDLength minimum ID length
	minIDLength = 3
	// maxImageNameLength
	maxImageNameLength = 72
	// maxImageDigstLength
	maxImageDigstLength = 64
	// digistPrefix
	digistPrefix = "@sha256:"
	// ErrParseImageID cannot parse an image ID
	ErrParseImageID = errors.New("cannot parse an image ID")
	// ErrRegistriesConfigure no registries configured
	ErrRegistriesConfigure = errors.New(`registries configured error`)
)

// HealthConfig means healtch check config in image
type HealthConfig struct {
	Test        []string      `json:",omitempty"`
	Interval    time.Duration `json:",omitempty"`
	Timeout     time.Duration `json:",omitempty"`
	StartPeriod time.Duration `json:",omitempty"`
	Retries     int           `json:",omitempty"`

	// Shut down a container if it becomes Unhealthy
	ExitOnUnhealthy bool `json:",omitempty"`
}

// ImageConfig means config in image
type ImageConfig struct {
	Healthcheck *HealthConfig
}

// ConfigFromJSON means config in json format
type ConfigFromJSON struct {
	Config ImageConfig `json:"config,omitempty"`
}

// ImageBasicSpec ImageBasicSpec
type ImageBasicSpec struct {
	ID           string
	Name         string
	RepoTags     []string
	RepoDigests  []string
	Size         *uint64
	Digest       digest.Digest
	ConfigDigest digest.Digest
	User         string
	Created      *time.Time `json:"created,omitempty"`
	// Loaded is the combined date and time at which the image was pulled, formatted as defined by RFC 3339, section 5.6.
	Loaded *time.Time `json:"Loaded,omitempty"`
}

type registryIndexInfo struct {
	name   string
	secure bool
}

type imageSummaryItem struct {
	user         string
	size         *uint64
	configDigest digest.Digest
}

type imageService struct {
	store                storage.Store
	defaultTransport     string
	insecureCIDRs        []*net.IPNet
	registryIndexConfigs map[string]*registryIndexInfo
	registries           []string
	ctx                  context.Context
}

type parsedImageNames struct {
	name                string
	secureSkipTLSVerify bool
}

// sizer knows its size.
type sizer interface {
	Size() (int64, error)
}

// ImageServer wraps up various implementation.
type ImageServer interface {
	// InitImage returns an Image
	InitImage(image parsedImageNames, options *copy.Options) (types.Image, error)
	// PullImage pull an image
	PullImage(systemContext *types.SystemContext, image parsedImageNames, dstImage string, options *copy.Options) (types.ImageReference, error)
	// CheckImages
	IntegrationCheck(systemContext *types.SystemContext) error
	// GetAllImages returns all images matches the filter
	GetAllImages(systemContext *types.SystemContext, filter string) ([]ImageBasicSpec, error)
	// GetOneImage returns an image matches the filter
	GetOneImage(systemContext *types.SystemContext, filter string) (*ImageBasicSpec, error)
	// UnrefImage reduce reference of the image
	UnrefImage(systemContext *types.SystemContext, imageName string) error
	// GetStore returns storage store
	GetStore() storage.Store
	// ParseImageNames parses an image
	ParseImageNames(imageName string) ([]parsedImageNames, error)
	// IsSecureIndex check if indexName is insecure
	IsSecureIndex(indexName string) bool
	// Tag image to other name
	Tag(srcName, destName string) error
}

func (svc *imageService) InitImage(image parsedImageNames, options *copy.Options) (types.Image, error) {
	srcRef, err := svc.initReference(image.name, image.secureSkipTLSVerify, options)
	if err != nil {
		return nil, err
	}

	srcCtx := &types.SystemContext{}
	if options.SourceCtx != nil {
		srcCtx = options.SourceCtx
	}
	return srcRef.NewImage(svc.ctx, srcCtx)
}

func (svc *imageService) PullImage(systemContext *types.SystemContext, image parsedImageNames, dstImage string, options *copy.Options) (types.ImageReference, error) {
	policy, err := signature.DefaultPolicy(systemContext)
	if err != nil {
		return nil, err
	}
	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		return nil, err
	}
	if options == nil {
		options = &copy.Options{}
	}

	srcRef, err := svc.initReference(image.name, image.secureSkipTLSVerify, options)
	if err != nil {
		return nil, err
	}

	destRef, err := svc.makeDestRef(dstImage)
	if err != nil {
		return nil, err
	}
	_, err = copy.Image(svc.ctx, policyContext, destRef, srcRef, options)
	if err != nil {
		return nil, err
	}
	return destRef, nil
}

func (svc *imageService) IntegrationCheck(systemContext *types.SystemContext) error {
	svc.store.GetCheckedLayers()
	defer svc.store.CleanupCheckedLayers()

	images, err := svc.store.Images()
	if err != nil {
		return err
	}
	for _, image := range images {
		logrus.Debugf("Try to check image %s", image.ID)
		err = svc.store.CheckImage(image.ID)
		if err != nil {
			logrus.Errorf("Delete image %s due to: %s", image.ID, err)
			err = svc.store.DeleteContainersByImage(image.ID)
			if err != nil {
				logrus.Errorf("Failed to delete all containers related to image %s with err: %s", image.ID, err)
			}
			err = svc.deleteImage(systemContext, image.ID)
			if err != nil {
				logrus.Errorf("Failed to delete image %s with err: %s", image.ID, err)
			}
		}
	}

	// Any container must based on an image now
	containers, err := svc.store.Containers()
	if err != nil {
		return err
	}
	for _, container := range containers {
		logrus.Debugf("Try to check container %s", container.ID)
		if !svc.store.Exists(container.ImageID) {
			logrus.Errorf("Delete container %s due to no related image found", container.ID)
			err = svc.store.DeleteContainer(container.ID)
			if err != nil {
				logrus.Errorf("Failed to delete container %s with err: %s", container.ID, err)
			}
		}
	}

	// Delete layers with no image related
	err = svc.store.DeleteUncheckedLayers()
	if err != nil {
		logrus.Errorf("Failed to delete unchecked layers: %v", err)
	}

	return nil
}

func (svc *imageService) GetAllImages(systemContext *types.SystemContext, filter string) ([]ImageBasicSpec, error) {
	if filter != "" {
		return svc.getAllImagesWithFilter(systemContext, filter)
	}

	return svc.getAllImagesWithoutFilter(systemContext)
}

func (svc *imageService) GetOneImage(systemContext *types.SystemContext, nameOrID string) (*ImageBasicSpec, error) {
	ref, err := svc.parseImageName(nameOrID)
	if err != nil {
		return nil, err
	}
	image, err := imstorage.Transport.GetStoreImage(svc.store, ref)
	if err != nil {
		return nil, err
	}
	imageFull, err := ref.NewImage(svc.ctx, systemContext)
	if err != nil {
		return nil, err
	}
	defer imageFull.Close()

	imageConfig, err := imageFull.OCIConfig(svc.ctx)
	if err != nil {
		return nil, err
	}

	img, err := ref.NewImageSource(svc.ctx, systemContext)
	if err != nil {
		return nil, err
	}
	defer img.Close()
	size := getImageSize(img)
	configDigest, err := getImageDigest(svc.ctx, img, nil)
	if err != nil {
		return nil, err
	}

	name, tags, digests := resortImageNames(image.Names)
	imageDigest, repoDigests := svc.getImageRepoDigests(digests, tags, image.ID)
	result := ImageBasicSpec{
		ID:           image.ID,
		Name:         name,
		RepoTags:     tags,
		RepoDigests:  repoDigests,
		Size:         size,
		Digest:       imageDigest,
		ConfigDigest: configDigest,
		User:         imageConfig.Config.User,
		Created:      &image.Created,
		Loaded:       &image.Loaded,
	}

	return &result, nil
}

func (svc *imageService) UnrefImage(systemContext *types.SystemContext, imageName string) error {
	ref, err := svc.parseImageName(imageName)
	if err != nil {
		return err
	}
	img, err := imstorage.Transport.GetStoreImage(svc.store, ref)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(img.ID, imageName) {
		namedRef, err := svc.initReference(imageName, false, &copy.Options{})
		if err != nil {
			return err
		}

		reducedNames := svc.getReducedNames(imageName, namedRef, img)
		if len(reducedNames) > 0 {
			return svc.store.SetNames(img.ID, reducedNames)
		}
	}

	return ref.DeleteImage(svc.ctx, systemContext)
}

func (svc *imageService) Tag(srcName, destName string) error {
	ref, err := svc.parseImageName(srcName)
	if err != nil {
		return err
	}
	img, err := imstorage.Transport.GetStoreImage(svc.store, ref)
	if err != nil {
		return err
	}

	return svc.store.AddName(img.ID, destName)
}

func (svc *imageService) GetStore() storage.Store {
	return svc.store
}

func (svc *imageService) IsSecureIndex(indexName string) bool {
	if index, ok := svc.registryIndexConfigs[indexName]; ok {
		return index.secure
	}

	host, _, err := net.SplitHostPort(indexName)
	if err != nil {
		host = indexName
	}

	addrs, err := net.LookupIP(host)
	if err != nil {
		ip := net.ParseIP(host)
		if ip != nil {
			addrs = []net.IP{ip}
		}

	}

	for _, addr := range addrs {
		for _, ipnet := range svc.insecureCIDRs {
			if ipnet.Contains(addr) {
				return false
			}
		}
	}

	return true
}

func (svc *imageService) ParseImageNames(imageName string) ([]parsedImageNames, error) {
	if len(imageName) >= minIDLength && svc.store != nil {
		if img, err := svc.store.Image(imageName); err == nil && img != nil && strings.HasPrefix(img.ID, imageName) {
			return []parsedImageNames{{img.ID, false}}, nil
		}
	}
	named, err := reference.ParseNormalizedNamed(imageName)
	if err != nil {
		if strings.Contains(err.Error(), "cannot specify 64-byte hexadecimal strings") {
			return nil, ErrParseImageID
		}
		return nil, err
	}
	domain, _ := parseDockerDomain(imageName)
	if domain != "" {
		return []parsedImageNames{{imageName, false}}, nil
	}
	if len(svc.registries) == 0 {
		return nil, fmt.Errorf("image %v has no domain and no registry-mirror found", imageName)
	}
	var images []parsedImageNames
	for _, r := range svc.registries {
		var image parsedImageNames
		if strings.HasPrefix(r, "http://") {
			image.secureSkipTLSVerify = true
		}
		r = strings.TrimPrefix(strings.TrimPrefix(r, "https://"), "http://")
		tagged, ok := reference.TagNameOnly(named).(reference.Tagged)
		if !ok {
			return nil, fmt.Errorf("Add tag for image %v failed", imageName)
		}
		image.name = path.Join(r, reference.Path(named)+":"+tagged.Tag())
		logrus.Debugf("before parse [%v], after parse [%v]", imageName, image.name)
		images = append(images, image)
	}
	return images, nil
}

func resortImageNames(imageNames []string) (firstName string, imageTags, imageDigests []string) {
	for _, image := range imageNames {
		if len(image) > maxImageNameLength && image[len(image)-maxImageNameLength:len(image)-maxImageDigstLength] == digistPrefix {
			imageDigests = append(imageDigests, image)
		} else {
			imageTags = append(imageTags, image)
		}
	}
	if len(imageDigests) > 0 {
		firstName = imageDigests[0]
	}
	if len(imageTags) > 0 {
		firstName = imageTags[0]
	}
	return firstName, imageTags, imageDigests
}

func (svc *imageService) parseImageName(imageName string) (types.ImageReference, error) {
	ref, err := alltransports.ParseImageName(imageName)
	if err != nil {
		ref2, err2 := imstorage.Transport.ParseStoreReference(svc.store, "@"+imageName)
		if err2 != nil {
			ref3, err3 := imstorage.Transport.ParseStoreReference(svc.store, imageName)
			if err3 != nil {
				return nil, err
			}
			ref2 = ref3
		}
		ref = ref2
	}
	return ref, nil
}

func (svc *imageService) getImageRepoDigests(oldRepoDigests, imageTags []string, imageID string) (imageDigest digest.Digest, repoDigests []string) {
	image, err := svc.store.Image(imageID)
	if err != nil {
		return "", oldRepoDigests
	}
	imageDigest = image.Digest
	if imageDigest == "" {
		imgDigest, err := svc.store.ImageBigDataDigest(imageID, storage.ImageDigestBigDataKey)
		if err != nil || imgDigest == "" {
			return "", oldRepoDigests
		}
		imageDigest = imgDigest
	}

	if len(imageTags) == 0 {
		return imageDigest, oldRepoDigests
	}

	digestMap := make(map[string]struct{})
	repoDigests = oldRepoDigests
	for _, repoDigest := range oldRepoDigests {
		digestMap[repoDigest] = struct{}{}
	}

	for _, tag := range imageTags {
		if ref, err2 := reference.ParseAnyReference(tag); err2 == nil {
			if name, ok := ref.(reference.Named); ok {
				trimmed := reference.TrimNamed(name)
				if imageRef, err3 := reference.WithDigest(trimmed, imageDigest); err3 == nil {
					if _, ok := digestMap[imageRef.String()]; !ok {
						repoDigests = append(repoDigests, imageRef.String())
						digestMap[imageRef.String()] = struct{}{}
					}
				}
			}
		}
	}
	return imageDigest, repoDigests
}

func (svc *imageService) getImageSummaryItem(systemContext *types.SystemContext, ref types.ImageReference) (imageSummaryItem, error) {
	img, err := ref.NewImageSource(svc.ctx, systemContext)
	if err != nil {
		return imageSummaryItem{}, err
	}
	size := getImageSize(img)
	configDigest, err := getImageDigest(svc.ctx, img, nil)
	img.Close()
	if err != nil {
		return imageSummaryItem{}, err
	}
	imageFull, err := ref.NewImage(svc.ctx, systemContext)
	if err != nil {
		return imageSummaryItem{}, err
	}
	defer imageFull.Close()
	imageConfig, err := imageFull.OCIConfig(svc.ctx)
	if err != nil {
		return imageSummaryItem{}, err
	}
	return imageSummaryItem{
		user:         imageConfig.Config.User,
		size:         size,
		configDigest: configDigest,
	}, nil
}

func (svc *imageService) appendSummaryResult(systemContext *types.SystemContext, ref types.ImageReference, image *storage.Image, results []ImageBasicSpec) ([]ImageBasicSpec, error) {
	var err error
	summaryItem, err := svc.getImageSummaryItem(systemContext, ref)
	if err != nil {
		return results, err
	}
	name, tags, digests := resortImageNames(image.Names)
	imageDigest, repoDigests := svc.getImageRepoDigests(digests, tags, image.ID)

	created := image.Created
	loaded := image.Loaded

	return append(results, ImageBasicSpec{
		ID:           image.ID,
		Name:         name,
		RepoTags:     tags,
		RepoDigests:  repoDigests,
		Size:         summaryItem.size,
		Digest:       imageDigest,
		ConfigDigest: summaryItem.configDigest,
		User:         summaryItem.user,
		Created:      &created,
		Loaded:       &loaded,
	}), nil
}

func (svc *imageService) getAllImagesWithFilter(systemContext *types.SystemContext, filter string) ([]ImageBasicSpec, error) {
	var results []ImageBasicSpec
	ref, err := svc.parseImageName(filter)
	if err != nil {
		return nil, err
	}
	if image, err := imstorage.Transport.GetStoreImage(svc.store, ref); err == nil {
		results, err = svc.appendSummaryResult(systemContext, ref, image, []ImageBasicSpec{})
		if err != nil {
			logrus.Errorf("Delete image %s due to: %s", image.ID, err)
			err = svc.store.DeleteContainersByImage(image.ID)
			if err != nil {
				logrus.Errorf("Failed to delete all containers related to image %s with err: %s", image.ID, err)
			}
			err = svc.deleteImage(systemContext, image.ID)
			if err != nil {
				logrus.Errorf("Failed to delete image %s with err: %s", image.ID, err)
			}
		}
	}
	return results, nil
}

func (svc *imageService) getAllImagesWithoutFilter(systemContext *types.SystemContext) ([]ImageBasicSpec, error) {
	var results []ImageBasicSpec
	images, err := svc.store.Images()
	if err != nil {
		return nil, err
	}
	for _, image := range images {
		ref, err := imstorage.Transport.ParseStoreReference(svc.store, "@"+image.ID)
		if err != nil {
			logrus.Errorf("Delete image %s due to: %s", image.ID, err)
			err = svc.store.DeleteContainersByImage(image.ID)
			if err != nil {
				logrus.Errorf("Failed to delete all containers related to image %s with err: %s", image.ID, err)
			}
			err = svc.deleteImage(systemContext, image.ID)
			if err != nil {
				logrus.Errorf("Failed to delete image %s with err: %s", image.ID, err)
			}
		}
		results, err = svc.appendSummaryResult(systemContext, ref, &image, results)
		if err != nil {
			logrus.Errorf("Delete image %s due to: %s", image.ID, err)
			err = svc.store.DeleteContainersByImage(image.ID)
			if err != nil {
				logrus.Errorf("Failed to delete all containers related to image %s with err: %s", image.ID, err)
			}
			err = svc.deleteImage(systemContext, image.ID)
			if err != nil {
				logrus.Errorf("Failed to delete image %s with err: %s", image.ID, err)
			}
		}
	}
	return results, nil
}

func getImageSize(img types.ImageSource) *uint64 {
	if s, ok := img.(sizer); ok {
		if sum, err := s.Size(); err == nil {
			usum := uint64(sum)
			return &usum
		}
	}
	return nil
}

func getImageDigest(ctx context.Context, image types.ImageSource, instanceDigest *digest.Digest) (digest.Digest, error) {
	imageManifestDatas, imageManifestType, err := image.GetManifest(nil, instanceDigest)
	if err != nil {
		return "", err
	}
	imageManifest, err := manifest.FromBlob(imageManifestDatas, imageManifestType)
	if err != nil {
		return "", err
	}
	return imageManifest.ConfigInfo().Digest, nil
}

// initReference init an image reference
func (svc *imageService) initReference(imageName string, secureSkipTLSVerify bool, options *copy.Options) (types.ImageReference, error) {
	if imageName == "" {
		return nil, storage.ErrNotAnImage
	}

	srcRef, err := alltransports.ParseImageName(imageName)
	if err != nil {
		if svc.defaultTransport == "" {
			return nil, err
		}
		srcRef2, err2 := alltransports.ParseImageName(svc.defaultTransport + imageName)
		if err2 != nil {
			return nil, err
		}
		srcRef = srcRef2
	}

	if options.SourceCtx == nil {
		options.SourceCtx = &types.SystemContext{}
	}

	if secureSkipTLSVerify {
		options.SourceCtx.DockerInsecureSkipTLSVerify = types.NewOptionalBool(true)
	} else {
		if srcRef.DockerReference() != nil {
			hostname := reference.Domain(srcRef.DockerReference())
			if secure := svc.IsSecureIndex(hostname); !secure {
				options.SourceCtx.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!secure)
			}
		}
	}

	return srcRef, nil
}

func (svc *imageService) makeDestRef(destImage string) (types.ImageReference, error) {
	return imstorage.Transport.ParseStoreReference(svc.store, destImage)
}

func (svc *imageService) getReducedNames(imageName string, namedRef types.ImageReference, img *storage.Image) []string {
	tmpName := imageName
	if namedRef.DockerReference() != nil {
		tmpName = namedRef.DockerReference().Name()
		if tagged, ok := namedRef.DockerReference().(reference.NamedTagged); ok {
			tmpName = tmpName + ":" + tagged.Tag()
		}
		if canonical, ok := namedRef.DockerReference().(reference.Canonical); ok {
			tmpName = tmpName + "@" + canonical.Digest().String()
		}
	}

	reducedNames := make([]string, 0, len(img.Names))
	for _, n := range img.Names {
		if n != tmpName && n != imageName {
			reducedNames = append(reducedNames, n)
		}
	}
	return reducedNames
}

func (svc *imageService) deleteImage(systemContext *types.SystemContext, imageName string) error {
	ref, err := svc.parseImageName(imageName)
	if err != nil {
		return err
	}
	return ref.DeleteImage(svc.ctx, systemContext)
}

func parseDockerDomain(imageName string) (domain, remainder string) {
	i := strings.IndexRune(imageName, '/')
	if i == -1 || (!strings.ContainsAny(imageName[:i], ".:") && imageName[:i] != "localhost") {
		domain, remainder = "", imageName
	} else {
		domain, remainder = imageName[:i], imageName[i+1:]
	}
	return
}

// InitImageService get the image service implementation.
func InitImageService(ctx context.Context, store storage.Store, defaultTransport string, insecureRegistries []string, registries []string) (ImageServer, error) {
	cleandRegistries := []string{}
	validRegistries := make(map[string]bool, len(registries))

	for _, i := range registries {
		if validRegistries[i] {
			continue
		}
		cleandRegistries = append(cleandRegistries, i)
		validRegistries[i] = true
	}

	if store == nil {
		var err error
		store, err = storage.GetStore(storage.DefaultStoreOptions)
		if err != nil {
			return nil, err
		}
	}

	newService := &imageService{
		store:                store,
		defaultTransport:     defaultTransport,
		registryIndexConfigs: make(map[string]*registryIndexInfo),
		insecureCIDRs:        make([]*net.IPNet, 0),
		registries:           cleandRegistries,
		ctx:                  ctx,
	}

	insecureRegistries = append(insecureRegistries, "127.0.0.0/8")
	for _, r := range insecureRegistries {
		_, ipnet, err := net.ParseCIDR(r)
		if err == nil {
			newService.insecureCIDRs = append(newService.insecureCIDRs, ipnet)
		} else {
			newService.registryIndexConfigs[r] = &registryIndexInfo{
				name:   r,
				secure: false,
			}
		}
	}

	return newService, nil
}
