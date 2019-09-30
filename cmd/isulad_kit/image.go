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
	"context"
	"errors"
	"net"
	"path"
	"strings"
	"time"

	"github.com/containers/image/copy"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/manifest"
	"github.com/containers/image/signature"
	istorage "github.com/containers/image/storage"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	digest "github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

const (
	minimumTruncatedIDLength = 3
)

var (
	// ErrCannotParseImageID cannot parse an image ID
	ErrCannotParseImageID = errors.New("cannot parse an image ID")
	// ErrImageMultiplyTagged image still has multiple names applied
	ErrImageMultiplyTagged = errors.New("image still has multiple names applied")
	// ErrNoRegistriesConfigured no registries configured
	ErrNoRegistriesConfigured = errors.New(`no registries configured while trying to pull an unqualified image, add at least one in /etc/crio/crio.conf under the "registries" key`)
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

// ImageResult ImageResult
type ImageResult struct {
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

type indexInfo struct {
	name   string
	secure bool
}

type imageSummaryItem struct {
	user         string
	size         *uint64
	configDigest digest.Digest
}

type imageService struct {
	store                 storage.Store
	defaultTransport      string
	insecureRegistryCIDRs []*net.IPNet
	indexConfigs          map[string]*indexInfo
	registries            []string
	ctx                   context.Context
}

// sizer knows its size.
type sizer interface {
	Size() (int64, error)
}

// ImageServer wraps up various implementation.
type ImageServer interface {
	// CheckImages
	CheckImages(systemContext *types.SystemContext) error
	// ListImages returns list of all images which match the filter.
	ListImages(systemContext *types.SystemContext, filter string) ([]ImageResult, error)
	// ImageStatus returns status of an image which matches the filter.
	ImageStatus(systemContext *types.SystemContext, filter string) (*ImageResult, error)
	// PrepareImage returns an Image where the config digest can be grabbed
	// for further analysis. Call Close() on the resulting image.
	PrepareImage(imageName string, options *copy.Options) (types.Image, error)
	// PullImage imports an image from the specified location.
	PullImage(systemContext *types.SystemContext, imageName string, options *copy.Options) (types.ImageReference, error)
	// UntagImage removes a name from the specified image, if it was
	// the only name the image had, removes the image.
	UntagImage(systemContext *types.SystemContext, imageName string) error
	// RemoveImage deletes the specified image.
	RemoveImage(systemContext *types.SystemContext, imageName string) error
	// GetStore returns the reference to the storage library
	GetStore() storage.Store
	// ResolveNames takes an image reference
	ResolveNames(imageName string) ([]string, error)
	// IsSecureIndex check if indexName is insecure
	IsSecureIndex(indexName string) bool
}

func (svc *imageService) getRef(name string) (types.ImageReference, error) {
	ref, err := alltransports.ParseImageName(name)
	if err != nil {
		ref2, err2 := istorage.Transport.ParseStoreReference(svc.store, "@"+name)
		if err2 != nil {
			ref3, err3 := istorage.Transport.ParseStoreReference(svc.store, name)
			if err3 != nil {
				return nil, err
			}
			ref2 = ref3
		}
		ref = ref2
	}
	return ref, nil
}

func sortNamesByType(names []string) (bestName string, tags, digests []string) {
	for _, name := range names {
		if len(name) > 72 && name[len(name)-72:len(name)-64] == "@sha256:" {
			digests = append(digests, name)
		} else {
			tags = append(tags, name)
		}
	}
	if len(digests) > 0 {
		bestName = digests[0]
	}
	if len(tags) > 0 {
		bestName = tags[0]
	}
	return bestName, tags, digests
}

func (svc *imageService) makeRepoDigests(knownRepoDigests, tags []string, imageID string) (imageDigest digest.Digest, repoDigests []string) {
	img, err := svc.store.Image(imageID)
	if err != nil {
		return "", knownRepoDigests
	}
	imageDigest = img.Digest
	if imageDigest == "" {
		imgDigest, err := svc.store.ImageBigDataDigest(imageID, storage.ImageDigestBigDataKey)
		if err != nil || imgDigest == "" {
			return "", knownRepoDigests
		}
		imageDigest = imgDigest
	}

	if len(tags) == 0 {
		return imageDigest, knownRepoDigests
	}

	digestMap := make(map[string]struct{})
	repoDigests = knownRepoDigests
	for _, repoDigest := range knownRepoDigests {
		digestMap[repoDigest] = struct{}{}
	}

	for _, tag := range tags {
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

func (svc *imageService) getImageSummaryItem(systemContext *types.SystemContext, ref types.ImageReference, image *storage.Image) (imageSummaryItem, error) {
	img, err := ref.NewImageSource(svc.ctx, systemContext)
	if err != nil {
		return imageSummaryItem{}, err
	}
	size := imageSize(img)
	configDigest, err := imageConfigDigest(svc.ctx, img, nil)
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

func (svc *imageService) appendSummaryResult(systemContext *types.SystemContext, ref types.ImageReference, image *storage.Image, results []ImageResult) ([]ImageResult, error) {
	var err error
	summaryItem, err := svc.getImageSummaryItem(systemContext, ref, image)
	if err != nil {
		return results, err
	}
	name, tags, digests := sortNamesByType(image.Names)
	imageDigest, repoDigests := svc.makeRepoDigests(digests, tags, image.ID)

	created := image.Created
	loaded := image.Loaded

	return append(results, ImageResult{
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

func (svc *imageService) CheckImages(systemContext *types.SystemContext) error {
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
			err = svc.RemoveImage(systemContext, image.ID)
			if err != nil {
				logrus.Errorf("Failed to delete image %s with err: %s", image.ID, err)
			}
		}
	}
	return nil
}

func (svc *imageService) listImagesWithFilter(systemContext *types.SystemContext, filter string) ([]ImageResult, error) {
	var results []ImageResult
	ref, err := svc.getRef(filter)
	if err != nil {
		return nil, err
	}
	if image, err := istorage.Transport.GetStoreImage(svc.store, ref); err == nil {
		results, err = svc.appendSummaryResult(systemContext, ref, image, []ImageResult{})
		if err != nil {
			logrus.Errorf("Delete image %s due to: %s", image.ID, err)
			err = svc.store.DeleteContainersByImage(image.ID)
			if err != nil {
				logrus.Errorf("Failed to delete all containers related to image %s with err: %s", image.ID, err)
			}
			err = svc.RemoveImage(systemContext, image.ID)
			if err != nil {
				logrus.Errorf("Failed to delete image %s with err: %s", image.ID, err)
			}
		}
	}
	return results, nil
}

func (svc *imageService) listImagesWithoutFilter(systemContext *types.SystemContext) ([]ImageResult, error) {
	var results []ImageResult
	images, err := svc.store.Images()
	if err != nil {
		return nil, err
	}
	for _, image := range images {
		ref, err := istorage.Transport.ParseStoreReference(svc.store, "@"+image.ID)
		if err != nil {
			logrus.Errorf("Delete image %s due to: %s", image.ID, err)
			err = svc.store.DeleteContainersByImage(image.ID)
			if err != nil {
				logrus.Errorf("Failed to delete all containers related to image %s with err: %s", image.ID, err)
			}
			err = svc.RemoveImage(systemContext, image.ID)
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
			err = svc.RemoveImage(systemContext, image.ID)
			if err != nil {
				logrus.Errorf("Failed to delete image %s with err: %s", image.ID, err)
			}
		}
	}
	return results, nil
}

func (svc *imageService) ListImages(systemContext *types.SystemContext, filter string) ([]ImageResult, error) {
	if filter != "" {
		return svc.listImagesWithFilter(systemContext, filter)
	}

	return svc.listImagesWithoutFilter(systemContext)
}

func (svc *imageService) ImageStatus(systemContext *types.SystemContext, nameOrID string) (*ImageResult, error) {
	ref, err := svc.getRef(nameOrID)
	if err != nil {
		return nil, err
	}
	image, err := istorage.Transport.GetStoreImage(svc.store, ref)
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
	size := imageSize(img)
	configDigest, err := imageConfigDigest(svc.ctx, img, nil)
	if err != nil {
		return nil, err
	}

	name, tags, digests := sortNamesByType(image.Names)
	imageDigest, repoDigests := svc.makeRepoDigests(digests, tags, image.ID)
	result := ImageResult{
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

func imageSize(img types.ImageSource) *uint64 {
	if s, ok := img.(sizer); ok {
		if sum, err := s.Size(); err == nil {
			usum := uint64(sum)
			return &usum
		}
	}
	return nil
}

func imageConfigDigest(ctx context.Context, img types.ImageSource, instanceDigest *digest.Digest) (digest.Digest, error) {
	manifestBytes, manifestType, err := img.GetManifest(nil, instanceDigest)
	if err != nil {
		return "", err
	}
	imgManifest, err := manifest.FromBlob(manifestBytes, manifestType)
	if err != nil {
		return "", err
	}
	return imgManifest.ConfigInfo().Digest, nil
}

// prepareReference creates an image reference from an image string and set options
// for the source context
func (svc *imageService) prepareReference(imageName string, options *copy.Options) (types.ImageReference, error) {
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

	if srcRef.DockerReference() != nil {
		hostname := reference.Domain(srcRef.DockerReference())
		if secure := svc.IsSecureIndex(hostname); !secure {
			options.SourceCtx.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!secure)
		}
	}

	return srcRef, nil
}

func (svc *imageService) PrepareImage(imageName string, options *copy.Options) (types.Image, error) {
	srcRef, err := svc.prepareReference(imageName, options)
	if err != nil {
		return nil, err
	}

	sourceCtx := &types.SystemContext{}
	if options.SourceCtx != nil {
		sourceCtx = options.SourceCtx
	}
	return srcRef.NewImage(svc.ctx, sourceCtx)
}

func (svc *imageService) getDestRefByImageName(srcRef types.ImageReference, imageName string) (types.ImageReference, error) {
	dest := imageName
	if srcRef.DockerReference() != nil {
		dest = srcRef.DockerReference().Name()
		if tagged, ok := srcRef.DockerReference().(reference.NamedTagged); ok {
			dest = dest + ":" + tagged.Tag()
		}
		if canonical, ok := srcRef.DockerReference().(reference.Canonical); ok {
			dest = dest + "@" + canonical.Digest().String()
		}
	}

	return istorage.Transport.ParseStoreReference(svc.store, dest)
}

func (svc *imageService) PullImage(systemContext *types.SystemContext, imageName string, options *copy.Options) (types.ImageReference, error) {
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

	srcRef, err := svc.prepareReference(imageName, options)
	if err != nil {
		return nil, err
	}

	destRef, err := svc.getDestRefByImageName(srcRef, imageName)
	if err != nil {
		return nil, err
	}
	_, err = copy.Image(svc.ctx, policyContext, destRef, srcRef, options)
	if err != nil {
		return nil, err
	}
	return destRef, nil
}

func (svc *imageService) getPrunedNames(nameOrID string, namedRef types.ImageReference, img *storage.Image) []string {
	name := nameOrID
	if namedRef.DockerReference() != nil {
		name = namedRef.DockerReference().Name()
		if tagged, ok := namedRef.DockerReference().(reference.NamedTagged); ok {
			name = name + ":" + tagged.Tag()
		}
		if canonical, ok := namedRef.DockerReference().(reference.Canonical); ok {
			name = name + "@" + canonical.Digest().String()
		}
	}

	prunedNames := make([]string, 0, len(img.Names))
	for _, imgName := range img.Names {
		if imgName != name && imgName != nameOrID {
			prunedNames = append(prunedNames, imgName)
		}
	}
	return prunedNames
}

func (svc *imageService) UntagImage(systemContext *types.SystemContext, nameOrID string) error {
	ref, err := svc.getRef(nameOrID)
	if err != nil {
		return err
	}
	img, err := istorage.Transport.GetStoreImage(svc.store, ref)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(img.ID, nameOrID) {
		namedRef, err := svc.prepareReference(nameOrID, &copy.Options{})
		if err != nil {
			return err
		}

		prunedNames := svc.getPrunedNames(nameOrID, namedRef, img)
		if len(prunedNames) > 0 {
			return svc.store.SetNames(img.ID, prunedNames)
		}
	}

	return ref.DeleteImage(svc.ctx, systemContext)
}

func (svc *imageService) RemoveImage(systemContext *types.SystemContext, nameOrID string) error {
	ref, err := svc.getRef(nameOrID)
	if err != nil {
		return err
	}
	return ref.DeleteImage(svc.ctx, systemContext)
}

func (svc *imageService) GetStore() storage.Store {
	return svc.store
}

func (svc *imageService) IsSecureIndex(indexName string) bool {
	if index, ok := svc.indexConfigs[indexName]; ok {
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
		for _, ipnet := range svc.insecureRegistryCIDRs {
			if ipnet.Contains(addr) {
				return false
			}
		}
	}

	return true
}

func splitDockerDomain(name string) (domain, remainder string) {
	i := strings.IndexRune(name, '/')
	if i == -1 || (!strings.ContainsAny(name[:i], ".:") && name[:i] != "localhost") {
		domain, remainder = "", name
	} else {
		domain, remainder = name[:i], name[i+1:]
	}
	return
}

func (svc *imageService) ResolveNames(imageName string) ([]string, error) {
	if len(imageName) >= minimumTruncatedIDLength && svc.store != nil {
		if img, err := svc.store.Image(imageName); err == nil && img != nil && strings.HasPrefix(img.ID, imageName) {
			return []string{img.ID}, nil
		}
	}
	_, err := reference.ParseNormalizedNamed(imageName)
	if err != nil {
		if strings.Contains(err.Error(), "cannot specify 64-byte hexadecimal strings") {
			return nil, ErrCannotParseImageID
		}
		return nil, err
	}
	domain, remainder := splitDockerDomain(imageName)
	if domain != "" {
		return []string{imageName}, nil
	}
	if len(svc.registries) == 0 {
		return nil, ErrNoRegistriesConfigured
	}
	images := []string{}
	for _, r := range svc.registries {
		rem := remainder
		if r == "docker.io" && !strings.ContainsRune(remainder, '/') {
			rem = "library/" + rem
		}
		images = append(images, path.Join(r, rem))
	}
	return images, nil
}

// GetImageService get the image service implementation.
func GetImageService(ctx context.Context, store storage.Store, defaultTransport string, insecureRegistries []string, registries []string) (ImageServer, error) {
	if store == nil {
		var err error
		store, err = storage.GetStore(storage.DefaultStoreOptions)
		if err != nil {
			return nil, err
		}
	}

	seenRegistries := make(map[string]bool, len(registries))
	cleanRegistries := []string{}
	for _, r := range registries {
		if seenRegistries[r] {
			continue
		}
		cleanRegistries = append(cleanRegistries, r)
		seenRegistries[r] = true
	}

	is := &imageService{
		store:                 store,
		defaultTransport:      defaultTransport,
		indexConfigs:          make(map[string]*indexInfo),
		insecureRegistryCIDRs: make([]*net.IPNet, 0),
		registries:            cleanRegistries,
		ctx:                   ctx,
	}

	insecureRegistries = append(insecureRegistries, "127.0.0.0/8")
	for _, r := range insecureRegistries {
		_, ipnet, err := net.ParseCIDR(r)
		if err == nil {
			is.insecureRegistryCIDRs = append(is.insecureRegistryCIDRs, ipnet)
		} else {
			is.indexConfigs[r] = &indexInfo{
				name:   r,
				secure: false,
			}
		}
	}

	return is, nil
}
