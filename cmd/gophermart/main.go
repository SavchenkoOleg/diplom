package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/SavchenkoOleg/diplom/internal/bonus"
	config "github.com/SavchenkoOleg/diplom/internal/conf"
	"github.com/SavchenkoOleg/diplom/internal/handlers"
	"github.com/SavchenkoOleg/diplom/internal/storage"
	"github.com/go-chi/chi/v5"
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
	CalcChanel := make(chan string, 100)
	conf.CalcChanel = CalcChanel
	defer func() { close(CalcChanel) }()

	// канал для сбора рассчитаных бонусов, к записи в БД
	UpChanel := make(chan config.UpdateOrderBonusStruct)
	conf.UpChanel = UpChanel
	defer func() { close(UpChanel) }()

	// сбор записей к рассчету
	ticker := time.NewTicker(100 * time.Millisecond)
	defer func() { ticker.Stop() }()

	go func() {
		for range ticker.C {
			bonus.ShelFindOrderToCalc(ctx, &conf)
		}

	}()

	// расчет и запись в БД
	go bonus.UpdateWorker(ctx, &conf)

	// сервер
	r := chi.NewRouter()
	r.Use(handlers.CompressGzip)
	r.Use(handlers.LogStdout)
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
