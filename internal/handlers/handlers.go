package handlers

import (
	"compress/gzip"
	"encoding/json"
	"log"
	"os"

	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/SavchenkoOleg/diplom/internal/conf"
	"github.com/SavchenkoOleg/diplom/internal/storage"
)

type compressBodyWr struct {
	http.ResponseWriter
	writer io.Writer
}

type stLoginPass struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// middleware

func DecompressGZIP(next http.Handler) http.Handler {
	// приводим возвращаемую функцию к типу функций HandlerFunc
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(`Content-Encoding`) == `gzip` { //	если входящий пакет сжат GZIP
			gz, err := gzip.NewReader(r.Body) //	изготавливаем reader-декомпрессор GZIP
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				log.Println("Request Body decompression error: " + err.Error())
				return
			}
			r.Body = gz //	подменяем стандартный reader из Request на декомпрессор GZIP
			defer gz.Close()
		}
		next.ServeHTTP(w, r) // передаём управление следующему обработчику
	})
}

func CompressGzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			r.Body = gz
			defer gz.Close()
		}
		next.ServeHTTP(w, r)

	})
}

func CookieMiddleware(conf *conf.Conf) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			cookieUserID, _ := r.Cookie("userID")

			if cookieUserID != nil {

				UserID := cookieUserID.Value
				cookie := http.Cookie{
					Name:   "userID",
					Value:  UserID,
					MaxAge: 3600}
				http.SetCookie(w, &cookie)

			}

			next.ServeHTTP(w, r)

		})
	}
}

func CheckAuthorizationMiddleware(conf *conf.Conf, arrNonAutorizedAPI []string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			apiPath := r.URL.Path

			for _, nPath := range arrNonAutorizedAPI {
				if strings.EqualFold(apiPath, nPath) {
					next.ServeHTTP(w, r)
					return
				}
			}

			cookieUserID, _ := r.Cookie("userID")

			if cookieUserID != nil {
				// проверить авторизацию
				userID := cookieUserID.Value

				Authorized, err := storage.IsUserAuthorized(r.Context(), conf, userID)
				if err != nil {
					http.Error(w, "uncorrect request format", 400)
					return
				}

				if !Authorized {
					http.Error(w, "not authorized", http.StatusUnauthorized)
					return
				}

				conf.UserID = userID
				next.ServeHTTP(w, r)
				return

			}

			http.Error(w, "not authorized", http.StatusUnauthorized)

		})
	}
}

// хендлеры
func HandlerRegister(conf *conf.Conf) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		fmt.Fprintln(os.Stdout, "В теле хендлера HandlerRegister")

		if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "uncorrect request format", 400)
			return
		}

		bodyIn := stLoginPass{}

		b, err := io.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		fmt.Fprintln(os.Stdout, "HandlerRegister прочитали bodyIn")

		if err := json.Unmarshal(b, &bodyIn); err != nil {
			http.Error(w, "uncorrect request format", 400)
			return
		}

		fmt.Fprintln(os.Stdout, "HandlerRegister размаршалили")

		if bodyIn.Login == "" || bodyIn.Password == "" {
			http.Error(w, "uncorrect request format", 400)
			return
		}

		fmt.Fprintln(os.Stdout, "HandlerRegister проверили JSON")

		code, err := storage.RegisterUser(r.Context(), conf, bodyIn.Login, bodyIn.Password)
		if err != nil {
			http.Error(w, "uncorrect request format", 400)
			return
		}

		fmt.Fprintln(os.Stdout, "HandlerRegister вернули 200")

		w.WriteHeader(code)

	}
}

func HandlerLogin(conf *conf.Conf) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "uncorrect request format", 400)
			return
		}
		bodyIn := stLoginPass{}

		b, err := io.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		if err := json.Unmarshal(b, &bodyIn); err != nil {
			http.Error(w, "uncorrect request format", 400)
			return
		}

		if bodyIn.Login == "" || bodyIn.Password == "" {
			http.Error(w, "uncorrect request format", 400)
			return
		}

		result, err := storage.LoginUser(r.Context(), conf, bodyIn.Login, bodyIn.Password)
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}

		if result.Code == 200 {
			cookie := http.Cookie{
				Name:   "userID",
				Value:  result.UserID,
				MaxAge: 3600}
			http.SetCookie(w, &cookie)
		}

		w.WriteHeader(result.Code)

	}
}

func HandlerNewOrder(conf *conf.Conf) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if !strings.Contains(r.Header.Get("Content-Type"), "text/plain") {
			http.Error(w, "uncorrect request format", 400)
			return
		}

		var oderNumber string

		b, err := io.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		oderNumber = string(b)
		defer r.Body.Close()

		oderInt64, err := strconv.ParseInt(oderNumber, 10, 64)
		if err != nil || !storage.LuhnValid(oderInt64) {
			http.Error(w, "uncorrect order number format", http.StatusUnprocessableEntity)
			return
		}

		resultCode, err := storage.AddNewOrder(r.Context(), conf, oderNumber)

		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}

		w.WriteHeader(resultCode)

	}
}

func HandlerUserOrdersList(conf *conf.Conf) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		result, err := storage.UserOrdersList(r.Context(), conf)

		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(result.Code)
		w.Write(result.OrdersList)
	}
}

func HandlerUserBalance(conf *conf.Conf) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		result, err := storage.UserBalance(r.Context(), conf)

		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(result.Code)
		w.Write(result.OrdersList)
	}
}

func HandlerUserWithdrawals(conf *conf.Conf) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		result, err := storage.UserWithdrawalsList(r.Context(), conf)

		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(result.Code)
		w.Write(result.OrdersList)
	}
}

func HandlerWithdraw(conf *conf.Conf) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "uncorrect request format", 500)
			return
		}

		type withdrawStruct struct {
			Order string
			Sum   float32
		}

		bodyIn := withdrawStruct{}

		b, err := io.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		if err := json.Unmarshal(b, &bodyIn); err != nil {
			http.Error(w, "uncorrect request format", 500)
			return
		}

		if bodyIn.Sum == 0 {
			http.Error(w, "uncorrect request format", 500)
			return
		}

		OrderInt64, err := strconv.ParseInt(bodyIn.Order, 10, 64)

		if err != nil || storage.LuhnValid(OrderInt64) {
			http.Error(w, "uncorrect order number format", http.StatusUnprocessableEntity)
			return
		}

		resultCode, err := storage.WithdrawSum(r.Context(), conf, bodyIn.Sum)
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}

		w.WriteHeader(resultCode)
	}
}
