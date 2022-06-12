package bonus

import (
	"context"
	"encoding/json"
	"fmt"
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

	selectText :=
		`SELECT odernumber 
	FROM orders as orders
	WHERE status IN('NEW','REGISTERED','PROCESSING') 
	ORDER BY orders.ordedate`

	rows, err := conf.PgxConnect.Query(ctx, selectText)

	if err != nil {
		return
	}
	defer rows.Close()

	var caclNubmers []string

	for rows.Next() {

		var number string
		if err := rows.Scan(&number); err != nil {
			return
		}

		caclNubmers = append(caclNubmers, number)

	}

	go func() { conf.CalcChanel <- caclNubmers }()
}

func ShelFindOrderToCalc(ctx context.Context, conf *config.Conf) {

	FindOrderToCalc(ctx, conf)
	RequestBonusCalculation(ctx, conf)

}

func RequestBonusCalculation(ctx context.Context, conf *config.Conf) {

	var arrOrderNubmer []string

	for numbers := range conf.CalcChanel {
		for _, number := range numbers {
			if notContains(arrOrderNubmer, number) {
				arrOrderNubmer = append(arrOrderNubmer, number)
			}

		}

		if len(conf.CalcChanel) == 0 {
			break
		}
	}

	for _, number := range arrOrderNubmer {

		CalcServAdr := conf.AccrualSystemAdress + "/api/orders/" + number

		log.Printf("Запрос В/К на адрес: %s", CalcServAdr)

		r, err := http.Get(CalcServAdr)
		if err != nil {
			log.Printf("Ошибка расчета В/К : %s", err.Error())
			return
		}

		b, err := io.ReadAll(r.Body)

		if err != nil {
			log.Printf("Ошибка чтение тела ответа В/К : %s", err.Error())
			return
		}
		defer r.Body.Close()

		log.Printf("Тело ответа : %s", string(b))

		var updateBonus config.UpdateOrderBonusStruct

		if err := json.Unmarshal(b, &updateBonus); err != nil {
			log.Printf("Ошибка Unmarshal тела ответа В/К : %s", err.Error())
			return
		}

		log.Printf("Расчет Order: %s", updateBonus.Order)
		log.Printf("Расчет Status: %s", updateBonus.Status)
		log.Printf("Расчет Accrual: %s", fmt.Sprintf("%f", updateBonus.Accrual))
		conf.UpChanel <- updateBonus

	}

}

func UpdateWorker(ctx context.Context, conf *config.Conf) {

	for rec := range conf.UpChanel {
		log.Printf("Запись в базу расчет Accrual: %s", rec.Order)
		updateBonusStatus(ctx, conf, rec)
	}

}

func updateBonusStatus(ctx context.Context, conf *config.Conf, rec config.UpdateOrderBonusStruct) {

	// открываем транзакцию
	tx, err := conf.PgxConnect.Begin(ctx)
	if err != nil {
		log.Printf("Ошибка начала транзакции обновления баланса : %s", err.Error())
		return
	}
	defer tx.Rollback(ctx)

	updateText := `UPDATE orders SET sum=$1, status=$2	WHERE odernumber = $3`
	_, err = tx.Exec(ctx, updateText, rec.Accrual, rec.Status, rec.Order)
	if err != nil {
		log.Printf("Ошибка запроса обновления баланса : %s", err.Error())
		return
	}

	// завершим транзакцию
	err = tx.Commit(ctx)
	if err != nil {
		log.Printf("Ошибка завершения транзакции обновления баланса : %s", err.Error())
		return
	}

}
