package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
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