package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/landjur/golibrary/uuid"
)

type Archive struct {
	SavePath string
	Name     string
	Key      string
	Expire   int64
}

var ArchiveStore map[string]Archive

// Given an string id searches for a file and delivers it to the client
func DownloadArchiveHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key, ok := vars["archiveKey"]

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "{\"error\": \"Not found\"}")
		return
	}

	archive, ok := ArchiveStore[key]

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "{\"error\": \"Not found\"}")
		return
	}

	if archive.Expire < time.Now().Unix() {
		// The archive have expire, clean it up!
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "{\"error\": \"Not found\"}")
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+archive.Name)
	http.ServeFile(w, r, archive.SavePath)
}

// Updates the file name and expire date
func UpdateArchiveHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := CheckAuth(r); !ok {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, "{\"error\": \"Unauthorized\"}")
		return
	}

	vars := mux.Vars(r)
	if _, ok := vars["archiveKey"]; ok {
		// TODO: Actually update the archive info
		fmt.Fprintf(w, "{\"status\": \"updated\"}")
		return
	}

	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, "{\"error\": \"Not found\"}")
}

// Deletes a file from the server
func DeleteArchiveHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := CheckAuth(r); !ok {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, "{\"error\": \"Unauthorized\"}")
		return
	}

	vars := mux.Vars(r)
	if key, ok := vars["archiveKey"]; ok {
		delete(ArchiveStore, key)
		// ToDo: Delete files from the server
		fmt.Fprintf(w, "{\"status\": \"deleted\"}")
		return
	}

	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, "{\"error\": \"Not found\"}")
}

// Lists all the files that the server has available to download
func ListArchiveHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := CheckAuth(r); !ok {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, "{\"error\": \"Unauthorized\"}")
		return
	}

	resp := map[string][]Archive{
		"archives": make([]Archive, 0),
	}

	for _, archive := range ArchiveStore {
		// Do not send the server path
		resp["archives"] = append(resp["archives"], archive)
	}

	json.NewEncoder(w).Encode(resp)
}

// Uploads a file to the server, returns the status and expire date
func AddArchiveHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := CheckAuth(r); !ok {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, "{\"error\": \"Unauthorized\"}")
		return
	}

	r.ParseForm()
	file, handler, err := r.FormFile("upload")
	if err != nil {
		// Bad Request
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "{\"error\": \"Bad Request\"}")
		return
	}
	defer file.Close()

	if handler != nil {
		key, err := uuid.NewV4()

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "{\"error\": \"Internal Server Error\"}")
			return
		}

		keyStr := key.String()
		expire := time.Now().Add(time.Hour * 24).Unix()

		r.ParseMultipartForm(32 << 20)
		// Read the save directory from Conf
		path := Conf["saveDir"] + "/" + keyStr

		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "{\"error\": \"Internal Server Error\"}")
			return
		}
		defer f.Close()

		io.Copy(f, file)

		ArchiveStore[keyStr] = Archive{
			Key:      keyStr,
			SavePath: path,
			Name:     handler.Filename,
			Expire:   expire,
		}

		resp := map[string]interface{}{
			"Name":   handler.Filename,
			"Key":    keyStr,
			"Expire": expire,
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	} else {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "{\"error\": \"Bad Request\"}")
		return
	}
}
