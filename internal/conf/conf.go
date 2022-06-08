package conf

import (
	"flag"
	"os"

	"github.com/jackc/pgx/v4"
)

type UpdateOrderBonusStruct struct {
	Order   int
	Status  string
	Accrual float32
}

type Conf struct {
	RunAdress           string
	DatabaseUri         string
	AccrualSystemAdress string
	PgxConnect          pgx.Conn
	UserID              string
	CalcChanel          chan int
	UpChanel            chan UpdateOrderBonusStruct
}

func ServiseConf() (outConf Conf) {

	// значениея по умолчанию
	outConf.RunAdress = "localhost:8080"
	outConf.DatabaseUri = "user=GoLogin password=gogo dbname=GoDB sslmode=disable"

	// переменные окружения
	RunAdress, exp := os.LookupEnv("RUN_ADDRESS")
	if exp {
		outConf.RunAdress = RunAdress
	}

	DatabaseUri, exp := os.LookupEnv("DATABASE_URI")
	if exp {
		outConf.DatabaseUri = DatabaseUri
	}

	AccrualSystemAdress, exp := os.LookupEnv("ACCRUAL_SYSTEM_ADDRESS ")
	if exp {
		outConf.AccrualSystemAdress = AccrualSystemAdress
	}

	// флаги
	flag.StringVar(&outConf.RunAdress, "a", outConf.RunAdress, "")
	flag.StringVar(&outConf.DatabaseUri, "d", outConf.DatabaseUri, "")
	flag.StringVar(&outConf.AccrualSystemAdress, "r", outConf.AccrualSystemAdress, "")
	flag.Parse()

	return outConf
}
