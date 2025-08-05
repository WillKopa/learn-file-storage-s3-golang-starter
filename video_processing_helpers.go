package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
)

func getVideoAspectRatio(filePath string) (string, error) {
	command := "ffprobe"
	params := []string{"-v", "error", "-print_format", "json", "-show_streams", filePath}
	cmd := exec.Command(command, params...)
	var buff bytes.Buffer
	cmd.Stdout = &buff
	err := cmd.Run()

	if err != nil {
		return "", fmt.Errorf("error running commands to get video aspect ratio: %s", err)
	}

	type aspect_ratio struct {
		Streams []struct {
			Width		int		`json:"width,omitempty"`
			Height		int		`json:"height,omitempty"`
		} 	`json:"streams"`
	}
	ar := aspect_ratio{}

	err = json.Unmarshal(buff.Bytes(), &ar)

	if err != nil {
		return "", fmt.Errorf("error reading meta data of video: %s", err)
	}
	precision := 100
	calculated_ratio := math.Round(float64(precision) * (float64(ar.Streams[0].Width)/float64(ar.Streams[0].Height))) / float64(precision)

	if calculated_ratio == 1.78 {
		return "16:9", nil
	} else if calculated_ratio == 0.56 {
		return "9:16", nil
	}
	return "other", nil

}

func processVideoForFastStart(filePath string) (string, error) {
	output_file_path := filePath + ".processing"
	command := "ffmpeg"
	args := []string{"-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", output_file_path}
	cmd := exec.Command(command, args...)
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error processing video for fast start: %s", err)
	}
	return output_file_path, nil
}

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	client := s3.NewPresignClient(s3Client)
	req, err := client.PresignGetObject(context.Background(), &s3.GetObjectInput{
		Bucket: &bucket,
		Key: &key,
	}, s3.WithPresignExpires(expireTime))
	if err != nil {
		return "", fmt.Errorf("error getting presigned client: %s", err)
	}
	return req.URL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	if video.VideoURL == nil {
		return video, nil
	}
	url := strings.Split(*video.VideoURL, ",")
	if len(url) < 2 {
		return video, nil
	}
	video_url, err := generatePresignedURL(cfg.s3Client, url[0], url[1], 5 * time.Minute)
	if err != nil {
		return video, fmt.Errorf("error generating presigned url: %s", err)
	}
	video.VideoURL = &video_url
	return video, nil
}