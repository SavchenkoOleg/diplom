package bonus

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"

	config "github.com/SavchenkoOleg/diplom/internal/conf"
)

func notContains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return false
		}
	}
	return true
}

func FindOrderToCalc(ctx context.Context, conf *config.Conf) {

	maxQuantityOrderForCalc := cap(conf.CalcChanel) - len(conf.CalcChanel)

	if maxQuantityOrderForCalc == 0 {
		return
	}

	selectText :=
		`SELECT odernumber 
	FROM orders as orders
	WHERE status IN('NEW','REGISTERED','PROCESSING') 
	ORDER BY orders.ordedate LIMIT $1`

	rows, err := conf.PgxConnect.Query(ctx, selectText, maxQuantityOrderForCalc)

	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {

		var number string
		if err := rows.Scan(&number); err != nil {
			return
		}

		if cap(conf.CalcChanel) == len(conf.CalcChanel) {
			break
		}
		conf.CalcChanel <- number
	}
}

func ShelFindOrderToCalc(ctx context.Context, conf *config.Conf) {

	FindOrderToCalc(ctx, conf)
	RequestBonusCalculation(ctx, conf)

}

func RequestBonusCalculation(ctx context.Context, conf *config.Conf) {

	var arrOrderNubmer []string

	for number := range conf.CalcChanel {
		if notContains(arrOrderNubmer, number) {
			arrOrderNubmer = append(arrOrderNubmer, number)
		}
		if len(conf.CalcChanel) == 0 {
			break
		}

	}

	for _, number := range arrOrderNubmer {

		CalcServAdr := conf.AccrualSystemAdress + number

		log.Printf("Запрос В/К на адрес: %s", CalcServAdr)

		r, err := http.Get(CalcServAdr)
		if err != nil {
			log.Printf("ошибка расчета В/К : %s", err.Error())
			return
		}

		b, err := io.ReadAll(r.Body)

		if err != nil {
			return
		}
		defer r.Body.Close()

		var updateBonus config.UpdateOrderBonusStruct

		if err := json.Unmarshal(b, &updateBonus); err != nil {
			return
		}

		conf.UpChanel <- updateBonus

	}

}

func UpdateWorker(ctx context.Context, conf *config.Conf) {

	for rec := range conf.UpChanel {
		updateBonusStatus(ctx, conf, rec)
	}

}

func updateBonusStatus(ctx context.Context, conf *config.Conf, rec config.UpdateOrderBonusStruct) {

	// открываем транзакцию
	tx, err := conf.PgxConnect.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)

	updateText := `UPDATE INTO orders(sum, status) VALUES ( $1, $2) WHERE odernumber = $3`
	_, err = tx.Exec(ctx, updateText, rec.Accrual, rec.Status, rec.Order)
	if err != nil {
		return
	}

	// завершим транзакцию
	err = tx.Commit(ctx)
	if err != nil {
		return
	}

}
