package main

import (
	"fmt"
	"net/http"
	"io"
	"os"
	"mime"
	
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

	fmt.Println("uploading video", videoID, "by user", userID)

	//maxmem still needs to be set here

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	mediaType := header.Header.Get("Content-Type")
	defer file.Close()

	// `file` is an `io.Reader` that we can read from to get the image data
	videoDetail, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Video not found", err)
		return
	}
	if videoDetail.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "User is not the owner of the video", nil)
		return
	}
	
	//convert mediaType to extension
	contentPart, _, err := mime.ParseMediaType(mediaType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "bad Content-Type", err)
		return
	}

	if contentPart != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "incorrect file type, only jpg and png allowed", nil)
		return
	}
	
	filename := getAssetPath(contentPart)


	fileDestination := cfg.getAssetDiskPath(filename)
	videoURL := cfg.getAssetURL(filename)

	video, err := os.Create(fileDestination)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to create file", err)
		return
	}

	_, err = io.Copy(video, file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to read file", err)
		return
	}

	videoDetail.VideoURL = &videoURL

	err = cfg.db.UpdateVideo(videoDetail)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to update Video Metadata", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoDetail)
}
