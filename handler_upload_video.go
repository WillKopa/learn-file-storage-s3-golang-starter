package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
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

	meta_data, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Video with given ID not found", err)
		return
	}

	if meta_data.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You cannot upload a video for someone else", nil)
		return
	}
	fmt.Println("uploading video", videoID, "by user", userID)

	const max_upload = 1 << 30
	http.MaxBytesReader(w, r.Body, max_upload)
	file, header, err := r.FormFile("video")
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
	mime_type := "video/mp4"
	if media_type != mime_type {
		respondWithError(w, http.StatusBadRequest, "Content-Type must be video/mp4", nil)
		return
	}

	temp_file_name := "tubely-upload.mp4"
	temp_file, err := os.CreateTemp("", temp_file_name)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error creating file for upload", err)
	}
	defer os.Remove(temp_file_name)
	defer temp_file.Close()

	_, err = io.Copy(temp_file, file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to read file", err)
		return
	}
	_, err = temp_file.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error resetting temp file pointer", err)
		return
	}

	ratio, err := getVideoAspectRatio(temp_file.Name())
	if err != nil {
		log.Printf("error: %s", err)
		respondWithError(w, http.StatusBadRequest, "Error getting video aspect ratio", err)
		return
	}
	prefix_key := "landscape"
	if ratio == "9:16" {
		prefix_key = "portrait"
	} else if ratio == "other" {
		prefix_key = "other"
	}
	prefix_key = "/" + prefix_key + "/"

	random_bytes := make([]byte, 32)
	rand.Read(random_bytes)
	param_key := prefix_key + base64.RawURLEncoding.EncodeToString(random_bytes) + ".mp4"
	params := s3.PutObjectInput{
		Bucket: &cfg.s3Bucket,
		Key: &param_key,
		Body: temp_file,
		ContentType: &mime_type,
	}
	cfg.s3Client.PutObject(r.Context(), &params)

	videoUrl := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, param_key)
	meta_data.VideoURL = &videoUrl

	err = cfg.db.UpdateVideo(meta_data)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error adding thumbnail to video", err)
		return
	}


	respondWithJSON(w, http.StatusOK, meta_data)
}
