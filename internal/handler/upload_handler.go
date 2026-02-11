package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"lusty/internal/middleware"
	"lusty/pkg/cloudinary"

	"github.com/gin-gonic/gin"
)

type UploadHandler struct {
	cloud cloudinary.Client
}

func NewUploadHandler(cloud cloudinary.Client) *UploadHandler {
	return &UploadHandler{cloud: cloud}
}

// UploadChatMedia allows any authenticated user to upload an image for chat. Returns URL.
func (h *UploadHandler) UploadChatMedia(c *gin.Context) {
	userID := middleware.GetUserID(c)
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file required"})
		return
	}
	folder := "Metchi/chat/" + strconv.FormatUint(uint64(userID), 10)
	publicID := "img_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:16]

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not read file"})
		return
	}
	defer f.Close()

	url, _, err := h.cloud.UploadImage(c.Request.Context(), f, folder, publicID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upload failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url})
}
