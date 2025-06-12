package main

import (
	"fmt"
	"net/http"
	"io"
	"os"
	"mime"
	"errors"
	"context"

	
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/aws"
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
	const maxMemory = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, maxMemory)
	r.ParseMultipartForm(maxMemory)

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

	tempVideo, err := os.CreateTemp("", "tubely_temp_vid.mp4")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to create file", err)
		return
	}
	defer os.Remove("tubely_temp_vid.mp4")
	defer tempVideo.Close()

	_, err = io.Copy(tempVideo, file)
	if err != nil {
	    if errors.Is(err, http.ErrBodyReadAfterClose) || err.Error() == "http: request body too large" {
	        respondWithError(w, http.StatusRequestEntityTooLarge, "Uploaded file is too large", err)
	        return
	    }
	    respondWithError(w, http.StatusBadRequest, "unable to read file", err)
	    return
	}
	//resedt temp file to beginning
	_, err = tempVideo.Seek(0, io.SeekStart)
	if err != nil {
	    respondWithError(w, http.StatusInternalServerError, "failed to reset file for upload", err)
	    return
	}

	//S3 upload
	_, err = cfg.s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
	    Bucket:      aws.String(cfg.s3Bucket),
	    Key:         aws.String(filename),
	    Body:        tempVideo,
	    ContentType: aws.String(contentPart),
	})
	if err != nil {
	    respondWithError(w, http.StatusInternalServerError, "failed to save file to bucket", err)
	    return
	}

	videoURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, filename)

	videoDetail.VideoURL = &videoURL

	err = cfg.db.UpdateVideo(videoDetail)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to update Video Metadata", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoDetail)
}
