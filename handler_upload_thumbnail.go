package main

import (
	"fmt"
	"io"
	"net/http"

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

	// STEP 2: Parse the form data
	const maxMemory = 10 << 20 // 10MB
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form", err)
		return
	}

	// STEP 3: Get the image data from the form
	file, fileHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to get file from form", err)
		return
	}
	defer file.Close()

	mediaType := fileHeader.Header.Get("Content-Type")

	// STEP 4: Read all the image data into a byte slice
	imageData, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to read file data", err)
		return
	}

	// STEP 5: Get the video's metadata and check ownership
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

	// STEP 6: Save the thumbnail to the global map
	thumbnailData := thumbnail{
		data:      imageData,
		mediaType: mediaType,
	}
	videoThumbnails[videoID] = thumbnailData

	// STEP 7: Update the video metadata with thumbnail URL
	thumbnailURL := fmt.Sprintf("http://localhost:%s/api/thumbnails/%s", cfg.port, videoID.String())
	
	// Update the video's thumbnail URL
	video.ThumbnailURL = &thumbnailURL
	err = cfg.db.UpdateVideo(video)  // Pass the video object, get back only error
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video", err)
		return
	}

	// STEP 8: Get the updated video from database and respond
	updatedVideo, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to get updated video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, updatedVideo)
}
