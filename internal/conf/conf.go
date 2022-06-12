package conf

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v4"
)

type UpdateOrderBonusStruct struct {
	Order   string
	Status  string
	Accrual float32
}

type Conf struct {
	RunAdress           string
	DatabaseURI         string
	AccrualSystemAdress string
	NonAutorizedAPI     []string
	PgxConnect          pgx.Conn
	UserID              string
	CalcChanel          chan []string
	UpChanel            chan UpdateOrderBonusStruct
	Ticker              *time.Ticker
}

func ServiseConf() (outConf Conf) {

	// значениея по умолчанию
	outConf.RunAdress = "localhost:8080"
	outConf.DatabaseURI = "user=GoLogin password=gogo dbname=GoDB sslmode=disable"

	// переменные окружения
	RunAdress, exp := os.LookupEnv("RUN_ADDRESS")
	if exp {
		outConf.RunAdress = RunAdress
	}

	DatabaseURI, exp := os.LookupEnv("DATABASE_URI")
	if exp {
		outConf.DatabaseURI = DatabaseURI
	}

	AccrualSystemAdress, exp := os.LookupEnv("ACCRUAL_SYSTEM_ADDRESS")
	if exp {
		outConf.AccrualSystemAdress = AccrualSystemAdress
	}

	log.Printf("**** EVENT ****** ")
	log.Printf("Установка outConf.RunAdress: %s", outConf.RunAdress)
	log.Printf("Установка outConf.DatabaseURI: %s", outConf.DatabaseURI)
	log.Printf("Установка outConf.AccrualSystemAdress: %s", outConf.AccrualSystemAdress)

	// флаги
	flag.StringVar(&outConf.RunAdress, "a", outConf.RunAdress, "")
	flag.StringVar(&outConf.DatabaseURI, "d", outConf.DatabaseURI, "")
	flag.StringVar(&outConf.AccrualSystemAdress, "r", outConf.AccrualSystemAdress, "")
	flag.Parse()

	log.Printf("**** FLAG ****** ")
	log.Printf("Установка outConf.RunAdress: %s", outConf.RunAdress)
	log.Printf("Установка outConf.DatabaseURI: %s", outConf.DatabaseURI)
	log.Printf("Установка outConf.AccrualSystemAdress: %s", outConf.AccrualSystemAdress)

	// EndPoint'ы, не требующие авторизации
	outConf.NonAutorizedAPI = []string{
		"/api/user/login",
		"/api/user/register",
	}

	// канал для передачи номеров ордеров  к расчету начисления бонусов
	CalcChanel := make(chan []string)
	outConf.CalcChanel = CalcChanel

	// канал для сбора рассчитаных бонусов, к записи в БД
	UpChanel := make(chan UpdateOrderBonusStruct)
	outConf.UpChanel = UpChanel

	// тикер фоновой задачи
	outConf.Ticker = time.NewTicker(100 * time.Millisecond)

	return outConf
}

func AllDefer(ctx context.Context, conf *Conf) {

	func() { conf.PgxConnect.Close(ctx) }()
	func() { conf.Ticker.Stop() }()
	func() { close(conf.CalcChanel) }()
	func() { close(conf.UpChanel) }()

}
