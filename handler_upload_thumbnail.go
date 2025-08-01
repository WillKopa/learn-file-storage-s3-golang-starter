package main

import (
	"encoding/base64"
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


	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const max_memory = 10 << 20
	err = r.ParseMultipartForm(max_memory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	media_type := header.Header.Get("Content-Type")
	data, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to read file", err)
		return
	}
	meta_data, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Video with given ID not found", err)
		return
	}

	if meta_data.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You cannot upload a thumbnail for someone else", nil)
		return
	}

	thumbnail_url := fmt.Sprintf("http://localhost:%s/api/thumbnails/%v", cfg.port, videoID)
	meta_data.ThumbnailURL = &thumbnail_url

	encoded_media_type := base64.StdEncoding.EncodeToString([]byte(media_type))
	encoded_data := base64.StdEncoding.EncodeToString([]byte(data))
	data_url := fmt.Sprintf("data:%s;base64,%s", encoded_media_type, encoded_data)
	meta_data.ThumbnailURL = &data_url

	err = cfg.db.UpdateVideo(meta_data)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error adding thumbnail to video", err)
		return
	}


	respondWithJSON(w, http.StatusOK, meta_data)
}
