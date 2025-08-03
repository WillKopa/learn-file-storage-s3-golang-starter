package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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

	media_type, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}
	if media_type != "image/jpeg" && media_type != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Content-Type must be image/jpeg or image/png", nil)
		return
	}

	extension := strings.ReplaceAll(media_type, "/", ".")
	random_bytes := make([]byte, 32)
	rand.Read(random_bytes)
	thumbnail_path := filepath.Join(cfg.assetsRoot, base64.RawURLEncoding.EncodeToString(random_bytes))
	thumbnail_path += fmt.Sprintf(".%s", extension)
	new_file, err := os.Create(thumbnail_path)
	thumbnail_url := fmt.Sprintf("http://localhost:%s/%s", cfg.port, thumbnail_path)
	// This is server side cache busting
	// thumbnail_url := fmt.Sprintf("http://localhost:%s/%s?v=%d", cfg.port, thumbnail_path, time.Now().Unix())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to save thumbnail", err)
		return
	}
	_, err = io.Copy(new_file, file)
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

	meta_data.ThumbnailURL = &thumbnail_url


	err = cfg.db.UpdateVideo(meta_data)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error adding thumbnail to video", err)
		return
	}


	respondWithJSON(w, http.StatusOK, meta_data)
}
