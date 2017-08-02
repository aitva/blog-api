package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/ulule/limiter"
)

var (
	errUnknownID    = errors.New("unknown ID")
	errUnknownTitle = errors.New("unknown title")
)

type article struct {
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type server struct {
	db  *bolt.DB
	mux *mux.Router
}

func main() {
	var err error
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	srv := &server{}
	srv.db, err = bolt.Open("blog.db", 0666, nil)
	if err != nil {
		log.Fatal(err)
	}

	store := limiter.NewMemoryStore()
	limit := limiter.NewLimiter(store, limiter.Rate{
		Period: 1 * time.Minute,
		Limit:  int64(264),
	})
	httpLimit := limiter.NewHTTPMiddleware(limit)

	srv.mux = mux.NewRouter()
	srv.mux.HandleFunc("/", srv.notFoundHandler)
	srv.mux.HandleFunc("/article/{id}/{title}/", srv.getArticleHandler).Methods("GET")
	srv.mux.HandleFunc("/article/{id}/", srv.postArticleHandler).Methods("POST")
	srv.mux.HandleFunc("/articles/{id}/", srv.getArticlesHandler)
	srv.mux.HandleFunc("/articles/{id}/{sort}", srv.getArticlesHandler)
	h := httpLimit.Handler(srv.mux)
	h = corsMiddleware(h)
	h = handlers.LoggingHandler(os.Stdout, h)
	http.ListenAndServe(":8080", h)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(code)
	w.Write([]byte(msg))
}

func corsMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.Header().Add("Access-Control-Allow-Method", "POST, GET, OPTIONS")
		w.Header().Add("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func (s *server) notFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Access-Control-Allow-Origin", "*")
	writeError(w, http.StatusNotFound, "nothing here...")
}

func (s *server) postArticleHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id, ok := params["id"]
	if !ok || id == "" {
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
	article.Timestamp = time.Now()
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

func (s *server) getArticleHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id, ok := params["id"]
	if !ok || id == "" {
		writeError(w, http.StatusBadRequest, "user ID is missing")
		return
	}
	title, ok := params["title"]
	if !ok || title == "" {
		writeError(w, http.StatusBadRequest, "article title is missing")
		return
	}

	a := &article{}
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(id))
		if b == nil {
			return errUnknownID
		}
		data := b.Get([]byte(title))
		if data == nil {
			return errUnknownTitle
		}
		return gob.NewDecoder(bytes.NewReader(data)).Decode(a)
	})
	if err == errUnknownID || err == errUnknownTitle {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		log.Println(err)
		writeError(w, http.StatusInternalServerError, "fail to access DB")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(a)
}

func (s *server) getArticlesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, http.StatusBadRequest, "unexpected HTTP method")
		return
	}

	params := mux.Vars(r)
	id, ok := params["id"]
	if !ok || id == "" {
		writeError(w, http.StatusBadRequest, "user ID is missing")
		return
	}

	var articles []*article
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(id))
		if b == nil {
			return errUnknownID
		}
		return b.ForEach(func(k, v []byte) error {
			a := &article{}
			err := gob.NewDecoder(bytes.NewReader(v)).Decode(a)
			if err != nil {
				return err
			}
			articles = append(articles, a)
			return nil
		})
	})
	if err == errUnknownID {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		log.Println(err)
		writeError(w, http.StatusInternalServerError, "fail to access DB")
		return
	}

	order, ok := params["sort"]
	if ok {
		if order == "asc" {
			sort.Slice(articles, func(i, j int) bool { return articles[i].Timestamp.Before(articles[j].Timestamp) })
		} else if order == "desc" {
			sort.Slice(articles, func(i, j int) bool { return articles[i].Timestamp.After(articles[j].Timestamp) })
		} else {
			writeError(w, http.StatusBadRequest, "invalid sort parameter")
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(articles)
}
