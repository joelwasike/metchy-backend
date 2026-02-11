package cloudinary

import (
	"context"
	"fmt"
	"io"

	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/cloudinary/cloudinary-go/v2/config"
)

// Config holds Cloudinary credentials (from env or config).
type Config struct {
	CloudName string
	APIKey    string
	APISecret string
}

// Client wraps Cloudinary upload and URL generation with optimization.
type Client interface {
	UploadImage(ctx context.Context, file io.Reader, folder, publicID string) (url, thumbnailURL string, err error)
	UploadVideo(ctx context.Context, file io.Reader, folder, publicID string) (url, thumbnailURL string, err error)
	DeleteByURL(ctx context.Context, url string) error
}

// Optimized image params for fast frontend loading
const (
	ImageQuality    = "auto"
	ImageFetchFormat = "auto"
	ImageCrop       = "fill"
	ImageWidth      = 800
	ThumbWidth      = 200
)

// Video optimization
const (
	VideoQuality = "auto:low"
	VideoWidth   = 1280
)

// BuildOptimizedImageURL returns a Cloudinary URL with transformations for optimized delivery.
// Caller can use this for existing public IDs.
func BuildOptimizedImageURL(cloudName, publicID string, width int) string {
	if width <= 0 {
		width = ImageWidth
	}
	return fmt.Sprintf("https://res.cloudinary.com/%s/image/upload/q_auto,f_auto,w_%d,c_fill/%s",
		cloudName, width, publicID)
}

// BuildOptimizedVideoURL returns optimized video URL (e.g. for streaming).
func BuildOptimizedVideoURL(cloudName, publicID string) string {
	return fmt.Sprintf("https://res.cloudinary.com/%s/video/upload/q_auto:low,f_auto/%s",
		cloudName, publicID)
}

// Eager transformations for upload (single string per SDK)
const (
	imageEager = "q_auto,f_auto,w_800,c_fill"
	videoEager = "q_auto:low,f_auto,w_1280"
)
var eagerAsyncFalse = false

type clientImpl struct {
	cloudName string
	uploader  *uploader.API
}

// NewClient returns a Client. Caller must provide a configured cloudinary.Cloudinary instance
// and pass it via a factory; we use interface to avoid coupling to cloudinary-go in callers.
type UploadResult struct {
	URL          string
	ThumbnailURL string
	PublicID     string
}

// UploadImage uploads an image with eager optimizations (auto quality, format, resize).
func (c *clientImpl) UploadImage(ctx context.Context, file io.Reader, folder, publicID string) (url, thumbnailURL string, err error) {
	result, err := c.uploader.Upload(ctx, file, uploader.UploadParams{
		Folder:     folder,
		PublicID:   publicID,
		Eager:      imageEager,
		EagerAsync: &eagerAsyncFalse,
	})
	if err != nil {
		return "", "", err
	}
	url = result.SecureURL
	if len(result.Eager) > 0 {
		thumbnailURL = result.Eager[0].SecureURL
	}
	if thumbnailURL == "" {
		thumbnailURL = BuildOptimizedImageURL(c.cloudName, result.PublicID, ThumbWidth)
	}
	return url, thumbnailURL, nil
}

// UploadVideo uploads a video with eager optimization.
func (c *clientImpl) UploadVideo(ctx context.Context, file io.Reader, folder, publicID string) (url, thumbnailURL string, err error) {
	result, err := c.uploader.Upload(ctx, file, uploader.UploadParams{
		Folder:        folder,
		PublicID:      publicID,
		ResourceType:  "video",
		Eager:         videoEager,
		EagerAsync:    &eagerAsyncFalse,
	})
	if err != nil {
		return "", "", err
	}
	url = result.SecureURL
	if len(result.Eager) > 0 {
		thumbnailURL = result.Eager[0].SecureURL
	}
	if thumbnailURL == "" {
		thumbnailURL = fmt.Sprintf("https://res.cloudinary.com/%s/video/upload/so_0/%s.jpg", c.cloudName, result.PublicID)
	}
	return url, thumbnailURL, nil
}

// DeleteByURL is a placeholder; Cloudinary Admin API can delete by public_id derived from URL.
func (c *clientImpl) DeleteByURL(ctx context.Context, url string) error {
	// Extract public_id from URL and call Admin API if needed
	_ = url
	return nil
}

// NewClientFromParams builds a Client from Cloudinary cloud name, API key, and secret.
func NewClientFromParams(cloudName, apiKey, apiSecret string) (Client, error) {
	cfg, err := config.NewFromParams(cloudName, apiKey, apiSecret)
	if err != nil {
		return nil, err
	}
	up, err := uploader.NewWithConfiguration(cfg)
	if err != nil {
		return nil, err
	}
	return &clientImpl{
		cloudName: cloudName,
		uploader:  up,
	}, nil
}
