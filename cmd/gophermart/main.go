package main

import (
	"context"
	"log"
	"net/http"

	"github.com/SavchenkoOleg/diplom/internal/bonus"
	config "github.com/SavchenkoOleg/diplom/internal/conf"
	"github.com/SavchenkoOleg/diplom/internal/handlers"
	"github.com/SavchenkoOleg/diplom/internal/storage"
	"github.com/go-chi/chi/v5"
)

func main() {

	conf := config.ServiseConf()
	ctx := context.Background()

	defer config.AllDefer(ctx, &conf)

	// подключение к БД
	err := storage.InitDB(ctx, &conf)
	if err != nil {
		log.Fatal(err)
	}

	// запуск фонового рассчета начисленных бонусов
	bonus.StartCalculation(ctx, &conf)

	// запуск сервера
	r := chi.NewRouter()
	r.Use(handlers.CompressGzip)
	r.Use(handlers.LogStdout)
	r.Use(handlers.CheckAuthorizationMiddleware(&conf))

	r.Post("/api/user/register", handlers.HandlerRegister(&conf))
	r.Post("/api/user/login", handlers.HandlerLogin(&conf))
	r.Post("/api/user/orders", handlers.HandlerNewOrder(&conf))
	r.Get("/api/user/orders", handlers.HandlerUserOrdersList(&conf))
	r.Get("/api/user/balance", handlers.HandlerUserBalance(&conf))
	r.Get("/api/user/balance/withdrawals", handlers.HandlerUserWithdrawals(&conf))
	r.Post("/api/user/balance/withdraw", handlers.HandlerWithdraw(&conf))

	err = http.ListenAndServe(conf.RunAdress, r)
	if err != nil {
		log.Fatal(err)

	}

}
