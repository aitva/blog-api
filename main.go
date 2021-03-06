package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"encoding/xml"
	"errors"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
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
	Title     string    `json:"title" xml:"title"`
	Content   string    `json:"content" xml:"content"`
	Timestamp time.Time `json:"timestamp" xml:"timestamp"`
}

type server struct {
	db  *bolt.DB
	mux *mux.Router
}

func main() {
	var err error
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	addr := os.Getenv("BLOG_API_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	db := os.Getenv("BLOG_API_DB")
	if db == "" {
		db = "blog.db"
	}

	srv := &server{}
	srv.db, err = bolt.Open(db, 0666, nil)
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
	// Article handlers.
	srv.mux.HandleFunc("/article/{id}/{title}/", srv.getArticleHandler).Methods("GET")
	srv.mux.HandleFunc("/article/{id}/{title}/", srv.deleteArticleHandler).Methods("DELETE")
	srv.mux.HandleFunc("/article/{id}/", srv.postArticleHandler).Methods("POST")
	// Articles handlers.
	srv.mux.HandleFunc("/articles/{id}/", srv.getArticlesHandler).Methods("GET")
	srv.mux.HandleFunc("/articles/{id}/{sort}", srv.getArticlesHandler).Methods("GET")
	srv.mux.HandleFunc("/articles/{id}/", srv.deleteArticlesHandler).Methods("DELETE")
	h := httpLimit.Handler(srv.mux)
	h = corsMiddleware(h)
	h = handlers.LoggingHandler(os.Stdout, h)

	log.Println("listening on:", addr)
	log.Fatal(http.ListenAndServe(addr, h))
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(code)
	w.Write([]byte(msg))
}

func corsMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.Header().Add("Access-Control-Allow-Methods", "GET, POST, OPTIONS, DELETE")
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

	a := &article{}
	err := json.NewDecoder(r.Body).Decode(a)
	if err != nil {
		writeError(w, http.StatusBadRequest, "fail to parse JSON")
		return
	}
	a.Timestamp = time.Now()

	err = s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(id))
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		err = gob.NewEncoder(&buf).Encode(a)
		if err != nil {
			return err
		}
		err = b.Put([]byte(a.Title), buf.Bytes())
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		log.Println("fail to access DB:", err)
		writeError(w, http.StatusInternalServerError, "fail to access DB")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(a)
	if err != nil {
		log.Println("fail to encode article:", err)
		writeError(w, http.StatusInternalServerError, "fail to encode response")
		return
	}
}

func (s *server) getArticleHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id, ok := params["id"]
	if !ok || id == "" {
		writeError(w, http.StatusBadRequest, "missing ID")
		return
	}
	title, ok := params["title"]
	if !ok || title == "" {
		writeError(w, http.StatusBadRequest, "missing title")
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
		writeError(w, http.StatusNotFound, err.Error())
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

func (s *server) deleteArticleHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id, ok := params["id"]
	if !ok || id == "" {
		writeError(w, http.StatusBadRequest, "missing ID")
		return
	}
	title, ok := params["title"]
	if !ok || title == "" {
		writeError(w, http.StatusBadRequest, "missing title")
		return
	}

	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(id))
		if b == nil {
			return errUnknownID
		}
		data := b.Get([]byte(title))
		if data == nil {
			return errUnknownTitle
		}
		return b.Delete([]byte(title))
	})
	if err == errUnknownID || err == errUnknownTitle {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		log.Println(err)
		writeError(w, http.StatusInternalServerError, "fail to access DB")
		return
	}
}

func (s *server) getArticlesHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id, ok := params["id"]
	if !ok || id == "" {
		writeError(w, http.StatusBadRequest, "missing ID")
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
		writeError(w, http.StatusNotFound, err.Error())
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

	accept := r.Header.Get("Accept")
	asXML := strings.Contains(accept, "text/xml")
	asJSON := strings.Contains(accept, "application/json")
	if asXML && !asJSON {
		w.Header().Set("Content-Type", "text/xml")
		err = xml.NewEncoder(w).Encode(struct {
			XMLName  xml.Name
			Articles []*article `xml:"article"`
		}{
			XMLName:  xml.Name{Local: "articles"},
			Articles: articles,
		})
	} else {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(articles)
	}
	if err != nil {
		log.Println("encoding fail:", err)
		writeError(w, http.StatusInternalServerError, "encoding fail")
	}
}

func (s *server) deleteArticlesHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id, ok := params["id"]
	if !ok || id == "" {
		writeError(w, http.StatusBadRequest, "missing ID")
		return
	}

	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(id))
		if b == nil {
			return errUnknownID
		}
		return tx.DeleteBucket([]byte(id))
	})
	if err == errUnknownID {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		log.Println(err)
		writeError(w, http.StatusInternalServerError, "fail to access DB")
		return
	}
}
