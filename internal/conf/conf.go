package conf

import (
	"flag"
	"os"

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
	PgxConnect          pgx.Conn
	UserID              string
	CalcChanel          chan string
	UpChanel            chan UpdateOrderBonusStruct
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

	AccrualSystemAdress, exp := os.LookupEnv("ACCRUAL_SYSTEM_ADDRESS ")
	if exp {
		outConf.AccrualSystemAdress = AccrualSystemAdress
	}

	// флаги
	flag.StringVar(&outConf.RunAdress, "a", outConf.RunAdress, "")
	flag.StringVar(&outConf.DatabaseURI, "d", outConf.DatabaseURI, "")
	flag.StringVar(&outConf.AccrualSystemAdress, "r", outConf.AccrualSystemAdress, "")
	flag.Parse()

	return outConf
}
