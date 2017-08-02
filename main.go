package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"log"
	"net/http"

	"github.com/boltdb/bolt"
)

type article struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type server struct {
	db *bolt.DB
}

func main() {
	var err error
	srv := &server{}
	srv.db, err = bolt.Open("blog.db", 0666, nil)
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/article/", srv.articleHandler)
	http.ListenAndServe(":8080", nil)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(code)
	w.Write([]byte(msg))
}

func (s *server) articleHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
	case "POST":
		s.postArticle(w, r)
	}
}

func (s *server) postArticle(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	id := v.Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "user ID is missing")
		return
	}
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		writeError(w, http.StatusBadRequest, "invalid content-type")
		return
	}
	article := &article{}
	err := json.NewDecoder(r.Body).Decode(article)
	if err != nil {
		writeError(w, http.StatusBadRequest, "fail to parse JSON")
		return
	}
	err = s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(id))
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		err = gob.NewEncoder(&buf).Encode(article)
		if err != nil {
			return err
		}
		err = b.Put([]byte(article.Title), buf.Bytes())
		if err != nil {
			return err
		}
		return nil
	})
}
