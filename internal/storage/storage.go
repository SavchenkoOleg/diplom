package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/SavchenkoOleg/diplom/internal/conf"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v4"
)

type LoginResult struct {
	UserID string
	Code   int
}

type ordersResult struct {
	Code       int
	OrdersList []byte
}

func generateID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		log.Fatal(err)
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func min(x, y float32) float32 {
	if x < y {
		return x
	}
	return y
}

func LuhnValid(number int64) bool {
	return (number%10+checksum(number/10))%10 == 0
}

func checksum(number int64) int64 {
	var luhn int64

	for i := 0; number > 0; i++ {
		cur := number % 10

		if i%2 == 0 { // even
			cur = cur * 2
			if cur > 9 {
				cur = cur%10 + cur/10
			}
		}

		luhn += cur
		number = number / 10
	}
	return luhn % 10
}

func stringHash(inString string) (outString string) {

	src := []byte(inString)
	h := sha256.New()
	h.Write(src)
	outString = hex.EncodeToString(h.Sum(nil))
	return outString
}

func InitDB(ctx context.Context, conf *conf.Conf) (success bool, err error) {

	db, err := pgx.Connect(ctx, conf.DatabaseURI)
	if err != nil {
		return false, err
	}

	conf.PgxConnect = *db

	_, err = db.Exec(ctx, "Create table if not exists users( userID TEXT NOT NULL, login TEXT UNIQUE, hash TEXT NOT NULL)")
	if err != nil {
		return false, err
	}

	_, err = db.Exec(ctx, "Create table if not exists orders( userID TEXT, oderNumber TEXT NOT NULL, ordeDate TIMESTAMP with time zone, sum NUMERIC(15,2) NOT NULL , status TEXT NOT NULL)")
	if err != nil {

		return false, err
	}

	_, err = db.Exec(ctx, "Create table if not exists writingoff( userID TEXT, oderNumber TEXT NOT NULL, writingoffDate TIMESTAMP with time zone, sum NUMERIC(15,2) NOT NULL)")
	if err != nil {

		return false, err
	}

	return true, nil
}

func RegisterUser(ctx context.Context, conf *conf.Conf, login, password string) (result LoginResult, err error) {

	result, err = LoginUser(ctx, conf, login, password)

	if err != nil {
		result.UserID = ""
		result.Code = 500
		return result, err
	}

	if result.Code == 200 {
		return result, err
	}

	userID := generateID()
	userHash := stringHash(password)

	_, err = conf.PgxConnect.Exec(ctx,
		"INSERT INTO users (userID, login, hash) VALUES ($1, $2, $3)",
		userID,
		login,
		userHash)

	if err != nil {
		if pgerr, ok := err.(*pgconn.PgError); ok && pgerr.Code == pgerrcode.UniqueViolation {
			result.UserID = ""
			result.Code = 409
			return result, nil
		}
		result.UserID = ""
		result.Code = 500
		return result, err
	}

	result.UserID = userID
	result.Code = 200
	return result, err
}

func LoginUser(ctx context.Context, conf *conf.Conf, login, password string) (result LoginResult, err error) {

	userHash := stringHash(password)

	rows, err := conf.PgxConnect.Query(ctx, "SELECT userID FROM users WHERE login = $1 and hash = $2 LIMIT 1", login, userHash)

	if err != nil {
		result.UserID = ""
		result.Code = 500
		return result, err
	}
	defer rows.Close()

	if rows.Next() {

		if err := rows.Scan(&result.UserID); err != nil {
			result.UserID = ""
			result.Code = 500
			return result, err
		}

		result.Code = 200
		return result, err
	}

	result.UserID = ""
	result.Code = 401
	return result, err

}

func IsUserAuthorized(ctx context.Context, conf *conf.Conf, userID string) (success bool, err error) {

	rows, err := conf.PgxConnect.Query(ctx, "SELECT userID FROM users WHERE userID = $1  LIMIT 1", userID)

	if err != nil {
		return false, err
	}
	defer rows.Close()

	if rows.Next() {
		return true, err
	}
	return false, err
}

func AddNewOrder(ctx context.Context, conf *conf.Conf, oderNumber string) (resultCode int, err error) {

	rows, err := conf.PgxConnect.Query(ctx, "SELECT userID FROM orders WHERE oderNumber = $1  LIMIT 1", oderNumber)

	if err != nil {
		return 500, err
	}
	defer rows.Close()

	var addUserID string
	if rows.Next() {
		if err := rows.Scan(&addUserID); err != nil {
			return 500, err
		}

		if addUserID == conf.UserID {
			return 200, nil
		}
		return 409, nil
	}

	_, err = conf.PgxConnect.Exec(ctx, "INSERT INTO orders (userID, oderNumber, ordeDate, sum, status) VALUES ($1, $2, $3, $4, $5)",
		conf.UserID,
		oderNumber,
		time.Now(),
		0,
		"NEW")

	if err != nil {
		return 500, err
	}

	return 202, nil
}

func UserOrdersList(ctx context.Context, conf *conf.Conf) (result ordersResult, err error) {

	type orderRecord struct {
		OderNumber string    `json:"number"`
		Status     string    `json:"status"`
		Sum        float32   `json:"accrual,omitempty"`
		OrdeDate   time.Time `json:"uploaded_at" format:"2006-01-02T15:04:05Z07:00"`
	}

	var arrOrderRecord []orderRecord

	rows, err := conf.PgxConnect.Query(ctx, "SELECT oderNumber, status, sum, ordeDate FROM orders WHERE userID = $1  ORDER BY ordeDate", conf.UserID)

	if err != nil {
		return result, err
	}
	defer rows.Close()

	for rows.Next() {

		rec := orderRecord{}
		if err := rows.Scan(&rec.OderNumber, &rec.Status, &rec.Sum, &rec.OrdeDate); err != nil {
			return result, err
		}

		arrOrderRecord = append(arrOrderRecord, rec)
	}

	if len(arrOrderRecord) == 0 {
		result.Code = 204
		return result, nil
	}

	result.OrdersList, err = json.MarshalIndent(arrOrderRecord, "", "")
	if err != nil {
		return result, err
	}

	result.Code = 200
	return result, nil

}

func UserBalance(ctx context.Context, conf *conf.Conf) (result ordersResult, err error) {

	type BalanceRecordStruct struct {
		Sum       float32 `json:"current"`
		Withdrawn float32 `json:"withdrawn"`
	}

	var BalanceRecord BalanceRecordStruct

	selectText :=
		`SELECT 
		debet.sum - kredit.sum as sum,
		kredit.sum as Withdrawn
		FROM
		 ( SELECT  coalesce(SUM(t1.sum),0 ) as sum
			 FROM  orders AS t1 
				 WHERE t1.userID = $1) AS debet,
		 ( SELECT  coalesce(SUM(t2.sum),0 ) as sum
			FROM  writingoff AS t2 
				WHERE t2.userID = $1) AS kredit`

	rows, err := conf.PgxConnect.Query(ctx, selectText, conf.UserID)

	if err != nil {
		log.Printf("ошибка запроса баланса: %s", err.Error())
		return result, err
	}
	defer rows.Close()

	for rows.Next() {

		if err := rows.Scan(&BalanceRecord.Sum, &BalanceRecord.Withdrawn); err != nil {
			log.Printf("ошибка получения результатов запроса баланса: %s", err.Error())
			return result, err
		}

	}

	result.OrdersList, err = json.MarshalIndent(BalanceRecord, "", "")
	if err != nil {
		log.Printf("ошибка маршалинга результатов запроса баланса: %s", err.Error())
		return result, err
	}

	result.Code = 200
	return result, nil

}

func UserWithdrawalsList(ctx context.Context, conf *conf.Conf) (result ordersResult, err error) {

	type writingoffRecord struct {
		OderNumber     string    `json:"order"`
		Sum            float32   `json:"accrual"`
		WritingoffDate time.Time `json:"processed_at" format:"2006-01-02T15:04:05Z07:00"`
	}

	var WritingoffRecords []writingoffRecord

	rows, err := conf.PgxConnect.Query(ctx, "SELECT oderNumber, sum, writingoffDate FROM writingoff WHERE userID = $1  ORDER BY writingoffDate", conf.UserID)

	if err != nil {
		return result, err
	}
	defer rows.Close()

	for rows.Next() {

		rec := writingoffRecord{}
		if err := rows.Scan(&rec.OderNumber, &rec.Sum, &rec.WritingoffDate); err != nil {
			return result, err
		}
		WritingoffRecords = append(WritingoffRecords, rec)
	}

	if len(WritingoffRecords) == 0 {
		result.Code = 204
		return result, nil
	}

	result.OrdersList, err = json.MarshalIndent(WritingoffRecords, "", "")
	if err != nil {
		return result, err
	}

	result.Code = 200
	return result, nil

}

func WithdrawSum(ctx context.Context, conf *conf.Conf, requestedSum float32) (resultCode int, err error) {
	// запрос на списание средств

	type BalanceStruct struct {
		oderNumber string
		sum        float32
	}

	var sumResult float32
	var balance []BalanceStruct
	var BalanceRecord BalanceStruct

	// открываем транзакцию
	tx, err := conf.PgxConnect.Begin(ctx)
	if err != nil {
		log.Printf("ошибка conf.PgxConnect.Begin : %s", err.Error())
		return 500, err
	}
	defer tx.Rollback(ctx)

	// проверяем баланс
	selectText :=
		`SELECT *
	FROM
		(SELECT debet.oderNumber AS oderNumber,
				coalesce(debet.sum, 0) - coalesce(kredit.sum, 0) AS sum
		FROM
			(SELECT SUM(t1.sum) AS SUM,
					t1.oderNumber AS oderNumber
			FROM orders AS t1
			WHERE t1.userID = $1
			GROUP BY t1.oderNumber) AS debet
		LEFT OUTER JOIN
			(SELECT coalesce(SUM(t2.sum), 0) AS SUM,
					t2.oderNumber AS oderNumber
			FROM writingoff AS t2
			WHERE t2.userID = $1
			GROUP BY t2.oderNumber) AS kredit ON debet.oderNumber= kredit.oderNumber) AS balance
	WHERE SUM>0`

	rows, err := tx.Query(ctx, selectText, conf.UserID)

	if err != nil {
		log.Printf("ошибка проверки баланса: %s", err.Error())
		return 500, err
	}
	defer rows.Close()

	for rows.Next() {

		if err := rows.Scan(&BalanceRecord.oderNumber, &BalanceRecord.sum); err != nil {
			return 500, err
		}

		sumResult += BalanceRecord.sum
		balance = append(balance, BalanceRecord)
	}

	if sumResult < requestedSum {
		// остаток бонусов минус списания меньше чем запррос к списанию
		// средств недостаточно
		return 402, nil
	}

	// распределим списание бонусов по остаткам
	var arrOrderNubber []string
	var arrWithdrawSum []float32

	rs := requestedSum
	for _, rec := range balance {
		minSum := min(rec.sum, rs)
		rec.sum -= minSum
		rs -= minSum
		arrOrderNubber = append(arrOrderNubber, rec.oderNumber)
		arrWithdrawSum = append(arrWithdrawSum, minSum)

		if rs == 0 {
			break
		}
	}

	// запишем списания в БД
	insertText :=
		`INSERT INTO writingoff(userid, odernumber, writingoffdate, sum)
	 	VALUES ( $1, unnest($2::TEXT[]), $3, unnest($4::NUMERIC[]))`
	_, err = tx.Exec(ctx, insertText,
		conf.UserID, arrOrderNubber, time.Now(), arrWithdrawSum)
	if err != nil {
		log.Printf("ошибка записи списания: %s", err.Error())
		return 500, err
	}

	// завершим транзакцию
	err = tx.Commit(ctx)
	if err != nil {
		log.Printf("ошибка закрытия транзакции: %s", err.Error())
		return 500, err
	}

	return 200, nil
}
