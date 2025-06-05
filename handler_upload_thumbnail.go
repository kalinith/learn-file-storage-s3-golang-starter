package main

import (
	"fmt"
	"net/http"
	"io"
	"os"
	"strings"
	
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
		respondWithError(w, http.StatusUnauthorized, "Not Own Video", err)
		return
	}

	//convert mediaType to extension??

	contentParts := strings.Split(mediaType, ";")//this should return up to 3 entries
	//for Example “image/png; charset=utf-8” will result in {"image/png", "charset=utf-8"}
	if len(contentParts) == 0 {
		//is it even possible to get to this point
		respondWithError(w, http.StatusBadRequest, "no content type provided", err)
		return
	}
	mediaParts := strings.Split(contentParts[0], "/")//this should return 2 entries
	//for Example “image/png” will result in {"image", "png"} and
	//“application/octet-stream” will result in {"application", "octet-stream"}
	if mediaParts[0] != "image" || len(mediaParts) < 2 {
		//if the first part isn't of type image it is not a valid thumbnail
		respondWithError(w, http.StatusBadRequest, "incorrect file type", err)
		return
	}
	extension := mediaParts[len(mediaParts)-1]

	//cfg.assetsRoot is the path for assets
	filename := fmt.Sprintf("%s.%s", videoIDString, extension)

	fileDestination := filepath.Join(cfg.assetsRoot, filename)

	image, err := os.Create(fileDestination)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to create file", err)
		return
	}

	_, err = io.Copy(image, file)
	//image, err := io.ReadAll(file)
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
