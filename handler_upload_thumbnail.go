package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	// Parse the form data
	const maxMemory = 10 << 20 // 10MB
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form", err)
		return
	}

	// Get the image data from the form
	file, fileHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to get file from form", err)
		return
	}
	defer file.Close()

	mediaType := fileHeader.Header.Get("Content-Type")

	// Get the video's metadata and check ownership
	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Video not found", err)
		return
	}

	// Check if the authenticated user owns this video
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You can only upload thumbnails for your own videos", nil)
		return
	}

	// STEP 1A: Determine file extension from Content-Type
	var fileExtension string
	switch mediaType {
	case "image/jpeg":
		fileExtension = ".jpg"
	case "image/png":
		fileExtension = ".png"
	case "image/gif":
		fileExtension = ".gif"
	case "image/webp":
		fileExtension = ".webp"
	default:
		// Try to get extension from filename as fallback
		if filename := fileHeader.Filename; filename != "" {
			fileExtension = filepath.Ext(filename)
		}
		if fileExtension == "" {
			respondWithError(w, http.StatusBadRequest, "Unsupported file type", nil)
			return
		}
	}

	// STEP 1B: Create unique file path using videoID
	filename := videoID.String() + fileExtension
	filePath := filepath.Join(cfg.assetsRoot, filename)

	// STEP 1C: Create the new file
	newFile, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create file", err)
		return
	}
	defer newFile.Close()

	// STEP 1D: Copy contents from multipart file to new file on disk
	_, err = io.Copy(newFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to save file", err)
		return
	}

	// STEP 2: Update the thumbnail_url to point to the file server
	thumbnailURL := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, filename)

	// Store the file URL in the database
	video.ThumbnailURL = &thumbnailURL
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video", err)
		return
	}

	// Get the updated video from database and respond
	updatedVideo, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to get updated video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, updatedVideo)
}
