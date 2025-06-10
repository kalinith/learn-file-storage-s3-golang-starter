package main

import (
	"fmt"
	"net/http"
	"io"
	"os"
	"mime"
	
	"path/filepath"
	"crypto/rand"
	"encoding/base64"

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
	//set up memory to store thumbnail
	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	// "thumbnail" should match the HTML form input name
	file, header, err := r.FormFile("thumbnail")
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
	//Step One, pull the media type out in the format x/y
	contentPart, _, err := mime.ParseMediaType(mediaType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "bad Content-Type", err)
		return
	}
	//step two, make sure the media type is image/jpeg or image/png
	if contentPart != "image/jpeg" && contentPart != "image/png" {
		respondWithError(w, http.StatusBadRequest, "incorrect file type, only jpg and png allowed", nil)
		return
	}
	//step three, check if the media type has a defined extension
	extensions, err := mime.ExtensionsByType(contentPart)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid Content-Type", err)
		return
	}
	if len(extensions) == 0 {
		respondWithError(w, http.StatusBadRequest, "No associated file type", nil)
		return
	}
	extension := extensions[0] //at this point we can narrow down to acceptable extensions

	//get a unique filename for the thumbnail
	key := make([]byte, 32)
	_, err = rand.Read(key)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Issue generating random name", err)
		return
	}
	var fileUniquID string
	fileUniquID = base64.RawURLEncoding.EncodeToString(key)
	filename := fmt.Sprintf("%s%s", fileUniquID, extension)
	fmt.Printf("unique:%s\nfile name:%s",fileUniquID, filename)
	fileDestination := filepath.Join(cfg.assetsRoot, filename)

	image, err := os.Create(fileDestination)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to create file", err)
		return
	}

	_, err = io.Copy(image, file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to read file", err)
		return
	}
	thumbnailURL := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, filename)
	videoDetail.ThumbnailURL = &thumbnailURL

	err = cfg.db.UpdateVideo(videoDetail)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to update Video Metadata", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoDetail)
}
