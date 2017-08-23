package main

import (
	"fmt"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fpe/fpe"
	"log"
	"net/http"
	"os"
	"strings"
	"io/ioutil"
	"path/filepath"

	"bitbucket.org/liamstask/goose/lib/goose"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	_ "github.com/go-sql-driver/mysql"
	"github.com/goware/cors"
	"github.com/unrolled/secure"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/kms"
)

// The RequestValues type describes the structure of the body of POST requests.
// The structure is json of this structure:
// {
//   "values": ["message", "values"],
//   "tweaks": ["abcdefgh", "12345678"]
// }
type RequestValues struct {
	Values []string `json:"values"`
	Tweaks []string `json:"tweaks"`
}

// The ResponseValues type describes the structure of the all responses.
// The structure is json of this structure:
// {
//   "values": ["message", "values"]
// }
type ResponseValues struct {
	Values []string `json:"values"`
}

var arks = make(map[string]fpe.Algorithm)
var dbConf goose.DBConf
var decryptedKey string

func getValuesFromURLParam(r *http.Request) ([]string, [][]byte, error) {
	values := r.URL.Query()["q"]
	if len(values) == 1 {
		values = strings.Split(values[0], ",")
	}

	tweakStrings := r.URL.Query()["tweaks"]
	if len(tweakStrings) == 1 {
		tweakStrings = strings.Split(tweakStrings[0], ",")
	}

	tweaks := make([][]byte, len(tweakStrings))
	for i := 0; i < len(tweakStrings); i++ {
		var err error
		tweaks[i], err = hex.DecodeString(tweakStrings[i])
		if err != nil {
			return nil, nil, err
		}
	}

	return values, tweaks, nil
}

func getValuesFromBody(r *http.Request) (RequestValues, error) {
	decoder := json.NewDecoder(r.Body)
	var requestValues RequestValues
	err := decoder.Decode(&requestValues)
	defer r.Body.Close()
	return requestValues, err
}

// GetEncryptHandler handles requests for GET /v1/ark/{arkname}/encrypt
// Takes a query parameter 'q' that is a comma separated list of values to encrypt
// and returns a response body of type ResponseValues.
func GetEncryptHandler(w http.ResponseWriter, r *http.Request) {
	ark := arks[chi.URLParam(r, "arkName")]
	values, tweaks, err := getValuesFromURLParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	payload := ResponseValues{Values: []string{}}
	for i := 0; i < len(values); i++ {
		value := values[i]
		tweak := []byte{}
		if i < len(tweaks) {
			tweak = tweaks[i]
		}
		message, err := ark.Encrypt(string(value), tweak)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		payload.Values = append(payload.Values, strings.ToUpper(message))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(payload)
	return
}

// PostEncryptHandler handles requests for POST /v1/ark/{arkname}/encrypt
// Takes a json body of structure RequestValues and returns a body of structure
// ResponseValues.
func PostEncryptHandler(w http.ResponseWriter, r *http.Request) {
	ark := arks[chi.URLParam(r, "arkName")]
	requestValues, err := getValuesFromBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	payload := ResponseValues{Values: []string{}}
	for i := 0; i < len(requestValues.Values); i++ {
		value := requestValues.Values[i]
		tweak := []byte{}
		if i < len(requestValues.Tweaks) {
			tweak, err = hex.DecodeString(requestValues.Tweaks[i])
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
		}
		message, err := ark.Encrypt(string(value), tweak)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		payload.Values = append(payload.Values, strings.ToUpper(message))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(payload)
	return
}

// GetDecryptHandler handles requests for GET /v1/ark/{arkname}/decrypt
// Takes a query parameter 'q' that is a comma separated list of values to decrypt
// and returns a response body of type ResponseValues.
func GetDecryptHandler(w http.ResponseWriter, r *http.Request) {
	ark := arks[chi.URLParam(r, "arkName")]
	values, tweaks, err := getValuesFromURLParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	payload := ResponseValues{Values: []string{}}
	for i := 0; i < len(values); i++ {
		value := values[i]
		tweak := []byte{}
		if i < len(tweaks) {
			tweak = tweaks[i]
		}
		message, err := ark.Decrypt(string(value), tweak)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		payload.Values = append(payload.Values, strings.ToUpper(message))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(payload)
	return
}

// PostDecryptHandler handles requests for POST /v1/ark/{arkname}/decrypt
// Takes a json body of structure RequestValues and returns a body of structure
// ResponseValues.
func PostDecryptHandler(w http.ResponseWriter, r *http.Request) {
	ark := arks[chi.URLParam(r, "arkName")]
	requestValues, err := getValuesFromBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	payload := ResponseValues{Values: []string{}}
	for i := 0; i < len(requestValues.Values); i++ {
		value := requestValues.Values[i]
		tweak := []byte{}
		if i < len(requestValues.Tweaks) {
			tweak, err = hex.DecodeString(requestValues.Tweaks[i])
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
		}
		message, err := ark.Decrypt(string(value), tweak)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		payload.Values = append(payload.Values, strings.ToUpper(message))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(payload)
	return
}

// ArkCtx checks to make sure the arkName URL parameter is valid ARK name and returns
// a 404 if it cannot be found.
func ArkCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arkName := chi.URLParam(r, "arkName")
		found := findAlgorithm(arkName)
		if found {
			next.ServeHTTP(w, r)
		} else {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("ARK name not configured"))
		}
	})
}

// APIKeyValid checks to make sure the provided API key is valid and returns an error
// otherwise.
func APIKeyValid(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.Trim(r.Header.Get("Authorization"), "Bearer ")
		if key == "" {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("You need a valid token in your request."))
			return
		}

		db, err := goose.OpenDBFromDBConf(&dbConf)
		if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
		defer db.Close()

		var foundKey string // foundKey doesn't do anything atm, Scan requires an arg
		err = db.QueryRow("SELECT value FROM api_keys WHERE value=?", key).Scan(&foundKey)
		switch {
		case err == sql.ErrNoRows:
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Token could not be found. Are you sure you have the right token?"))
			return
		case err != nil:
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)
	w.Write([]byte(err.Error()))
}
// check arks to see if arkName already in memory, if not check db
// every db check will populate ark[arkName] if found in db.
// if not found in db, return false
func findAlgorithm(arkName string) bool {
	_, found := arks[arkName]
	if found {
		return true
	}

	db, err := goose.OpenDBFromDBConf(&dbConf)
	if err != nil { } // handle this error
	defer db.Close()

	var (name string; algorithmType string; keyString string; radix int;
			 minMessageLength int; maxMessageLength int; maxTweakLength int)

	err = db.QueryRow("SELECT * FROM arks WHERE ark_name=?", arkName).Scan(
		&name, &algorithmType, &keyString, &radix, &minMessageLength, &maxMessageLength,
		&maxTweakLength)
	if err != nil { return false }

	fmt.Println(name, algorithmType, keyString, radix, minMessageLength, maxMessageLength, maxTweakLength)

	if (strings.ToLower(algorithmType) == "ff1") {
		newAlgorithm, _ := fpe.NewFF1(decryptedKey, radix, minMessageLength, maxMessageLength, maxTweakLength)
		arks[name] = &newAlgorithm
	}else if (strings.ToLower(algorithmType) == "ff3") {
		newAlgorithm, _ := fpe.NewFF3(decryptedKey, radix, minMessageLength, maxMessageLength, maxTweakLength)
		arks[name] = &newAlgorithm
	}

	return true
}

func updateArks() {

}

func main() {
	conf, _ := goose.NewDBConf("db", "development", "")
	dbConf = *conf

	kmsKeyARN := "arn:aws:kms:us-west-2:302756457565:key/24e23158-2ba8-4f00-9a9a-94cae6018ca0"
  kmsClient, err := kms.New(session.New(&aws.Config{
    Region: aws.String("us-west-2"),
		Credentials: credentials.NewSharedCredentials("", "format-preserving-encryption"),
  }))

	absPath, err := filepath.Abs("./keyfile")
	if err != nil {
		log.Fatal(err)
	}

	serviceKeyBytes, err := ioutil.ReadFile(absPath)
	if err != nil {
		log.Fatal(err)
	}

	serviceKeyString := string(serviceKeyBytes)

	params := &kms.decryptInput{
		CiphertextBlob: serviceKeyString,
		EncryptionContext: {
			KeyId: "arn:aws:kms:us-west-2:302756457565:key/24e23158-2ba8-4f00-9a9a-94cae6018ca0"
		}
	}

	decryptedServiceKey, err := kmsClient.Decrypt(params)
	if err != nil {
		log.Fatal(err)
	}

	decryptedServiceKeyDecoded, err := base64.StdEncoding.DecodeString(decryptedServiceKey)
	if err != nil {
		log.Fatal(err)
	}

	decryptedKey = *decryptedServiceKeyDecoded
	// log.Print(decryptedServiceKey)
	// log.Print(decryptedServiceKeyDecoded)

	secureMiddleware := secure.New(secure.Options{
		FrameDeny:        true,
		BrowserXssFilter: true,
	})

	r := chi.NewRouter()

	cors := cors.New(cors.Options{
		// AllowedOrigins: []string{"https://foo.com"}, // Use this to allow specific origin hosts
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	})
	r.Use(cors.Handler)

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(secureMiddleware.Handler)

	r.Route("/v1/ark/{arkName}", func(r chi.Router) {
		r.Use(ArkCtx)
		r.Use(APIKeyValid)

		r.Get("/encrypt", GetEncryptHandler)
		r.Post("/encrypt", PostEncryptHandler)
		r.Get("/decrypt", GetDecryptHandler)
		r.Post("/decrypt", PostDecryptHandler)
	})

	f, _ := os.Create("/var/log/golang/fpe-server.log")
	defer f.Close()
	log.SetOutput(f)

	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}
	log.Printf("Listening on port %s\n\n", port)
	http.ListenAndServe(":"+port, r)
}
