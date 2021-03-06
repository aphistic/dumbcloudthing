package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/mux"
	"io/ioutil"
)

func main() {
	fmt.Printf("Listening on :3030\n")
	http.ListenAndServe(":3030", newHttpHandler())
}

type httpHandler struct {
	router *mux.Router
	pool   *redis.Pool
}

func newHttpHandler() *httpHandler {
	r := mux.NewRouter()
	h := &httpHandler{
		router: r,
		pool: &redis.Pool{
			Dial: dialRedis,
		},
	}

	r.HandleFunc("/ping", h.Ping).Methods("GET")
	r.HandleFunc("/health", h.Health).Methods("GET")
	r.HandleFunc("/kv/{key}", h.Get).Methods("GET")
	r.HandleFunc("/kv/{key}", h.Set).Methods("POST")
	r.HandleFunc("/code/{code}", h.Code).Methods("GET")
	r.HandleFunc("/exit/{code}", h.Exit).Methods("GET")

	return h
}

func (h *httpHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	h.router.ServeHTTP(w, req)
	req.Body.Close()
}

func (h *httpHandler) Ping(w http.ResponseWriter, req *http.Request) {
	fmt.Printf("Got ping from %s\n", req.RemoteAddr)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("PONG!"))
}

func (h *httpHandler) Health(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (h *httpHandler) Code(w http.ResponseWriter, req *http.Request) {
	fmt.Printf("Got code from %s\n", req.RemoteAddr)
	vars := mux.Vars(req)

	code, err := strconv.ParseUint(vars["code"], 10, 0)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Code is not a uint"))
		return
	}

	w.WriteHeader(int(code))
	w.Write([]byte(fmt.Sprintf("%d", code)))
}

func (h *httpHandler) Exit(w http.ResponseWriter, req *http.Request) {
	fmt.Printf("Got exit from %s\n", req.RemoteAddr)
	vars := mux.Vars(req)

	code, err := strconv.ParseInt(vars["code"], 10, 0)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Code is not an int"))
		return
	}

	os.Exit(int(code))
}

func (h *httpHandler) Get(w http.ResponseWriter, req *http.Request) {
	fmt.Printf("Got get from %s\n", req.RemoteAddr)
	conn := h.pool.Get()
	defer conn.Close()

	vars := mux.Vars(req)

	fmt.Printf("Doing redis GET on %s", vars["key"])
	res, err := conn.Do("GET", vars["key"])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	switch resData := res.(type) {
	case []byte:
		w.WriteHeader(200)
		w.Write(resData)
	default:
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("Unknown data type: %#v", res)))
	}
}

func (h *httpHandler) Set(w http.ResponseWriter, req *http.Request) {
	fmt.Printf("Got set from %s\n", req.RemoteAddr)
	conn := h.pool.Get()
	defer conn.Close()

	vars := mux.Vars(req)

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Error reading body: %s", err)))
		return
	}

	fmt.Printf("Doing redis SET on %s to:\n%s\n", vars["key"], body)
	_, err = conn.Do("SET", vars["key"], body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Error writing to redis: %s", err)))
		return
	}

	w.WriteHeader(200)
	w.Write([]byte(body))
}

func dialRedis() (redis.Conn, error) {
	var err error

	host := os.Getenv("REDIS_HOST")
	port := int64(6379)
	if os.Getenv("REDIS_PORT") != "" {
		port, err = strconv.ParseInt(os.Getenv("REDIS_PORT"), 10, 0)
		if err != nil {
			return nil, err
		}
	}
	pass := os.Getenv("REDIS_PASSWORD")
	db := int64(0)
	if os.Getenv("REDIS_DB") != "" {
		db, err = strconv.ParseInt(os.Getenv("REDIS_DB"), 10, 0)
		if err != nil {
			return nil, err
		}
	}

	opts := []redis.DialOption{
		redis.DialDatabase(int(db)),
	}
	if pass != "" {
		opts = append(opts, redis.DialPassword(pass))
	}

	fmt.Printf("Dialing %s:%d/%d\n", host, port, db)
	conn, err := redis.Dial(
		"tcp",
		fmt.Sprintf("%s:%d", host, port),
		opts...,
	)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Pinging redis\n")
	_, err = conn.Do("PING")
	if err != nil {
		return nil, err
	}

	fmt.Printf("Got redis connection\n")
	return conn, nil
}
