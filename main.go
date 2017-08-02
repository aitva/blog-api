package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
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
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	srv := &server{}
	srv.db, err = bolt.Open("blog.db", 0666, nil)
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/", srv.notFoundHandler)
	http.HandleFunc("/article/", srv.articleHandler)
	http.ListenAndServe(":8080", nil)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(code)
	w.Write([]byte(msg))
}

func (s *server) notFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Access-Control-Allow-Origin", "*")
	writeError(w, http.StatusNotFound, "nothing here...")
}

func (s *server) articleHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Method", "POST, GET, OPTIONS")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type")
	switch r.Method {
	case "GET":
		s.getArticle(w, r)
	case "POST":
		s.postArticle(w, r)
	case "OPTIONS":
		w.WriteHeader(200)
	default:
		writeError(w, http.StatusBadRequest, "unexpected HTTP method")
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
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fail to access DB")
		return
	}
}

func (s *server) getArticle(w http.ResponseWriter, r *http.Request) {
	unknownID := errors.New("user ID is unknown")
	v := r.URL.Query()
	id := v.Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "user ID is missing")
		return
	}
	title := v.Get("title")
	if title == "" {
		writeError(w, http.StatusBadRequest, "article title is missing")
		return
	}
	a := &article{}
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(id))
		if b == nil {
			return unknownID
		}
		data := b.Get([]byte(title))
		return gob.NewDecoder(bytes.NewReader(data)).Decode(a)
	})
	if err != nil {
		if err == unknownID {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Println(err)
		writeError(w, http.StatusInternalServerError, "fail to access DB")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(a)
}
