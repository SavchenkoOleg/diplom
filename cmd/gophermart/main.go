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
	"github.com/go-chi/chi/v5/middleware"
)

func main() {

	conf := config.ServiseConf()
	ctx := context.Background()

	nonAutorizedAPI := []string{
		"/api/user/login",
		"/api/user/register",
	}

	// подключение к БД
	if conf.DatabaseURI != "" {
		success, err := storage.InitDB(ctx, &conf)
		if err != nil {
			log.Fatal(err)

		}
		if success {
			defer conf.PgxConnect.Close(ctx)
		}
	}

	// канал для передачи номеров ордеров  к расчету начисления бонусов
	CalcChanel := make(chan int, 10)
	conf.CalcChanel = CalcChanel
	defer func() { close(CalcChanel) }()

	// канал для сбора рассчитаных бонусов, к записи в БД
	UpChanel := make(chan config.UpdateOrderBonusStruct)
	conf.UpChanel = UpChanel
	defer func() { close(UpChanel) }()

	// сбор записей к рассчету
	go bonus.StartFindOrderToCalc(ctx, &conf)

	// расчет и запись в БД
	go bonus.UpdateWorker(ctx, &conf)

	// сервер
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(handlers.CompressGzip)
	r.Use(handlers.CookieMiddleware(&conf))
	r.Use(handlers.CheckAuthorizationMiddleware(&conf, nonAutorizedAPI))

	r.Post("/api/user/register", handlers.HandlerRegister(&conf))
	r.Post("/api/user/login", handlers.HandlerLogin(&conf))
	r.Post("/api/user/orders", handlers.HandlerNewOrder(&conf))
	r.Get("/api/user/orders", handlers.HandlerUserOrdersList(&conf))
	r.Get("/api/user/balance", handlers.HandlerUserBalance(&conf))
	r.Get("/api/user/balance/withdrawals", handlers.HandlerUserWithdrawals(&conf))
	r.Post("/api/user/balance/withdraw", handlers.HandlerWithdraw(&conf))

	err := http.ListenAndServe(conf.RunAdress, r)
	if err != nil {
		log.Fatal(err)

	}

}
