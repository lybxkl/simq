package mySql

import (
	"database/sql"
	"errors"
	_ "github.com/go-sql-driver/mysql"
	"log"
)

type DB struct {
	link *sql.DB
	stmt *sql.Stmt
}

/**
  建立连接
*/
func (dd *DB) OpenLink(driverName string, dataSourceName string) *sql.DB {
	if dd.link != nil {
		log.Fatal(errors.New("mysql.Client: cannot reopen client"))
		return dd.link
	}
	db, err := sql.Open(driverName, dataSourceName) //注意严格区分大小写
	checkErr(err)
	return db
}

//预先执行
func (dd *DB) Prepare(db *sql.DB, str string) {
	stmt, err := db.Prepare(str)
	checkErr(err)
	dd.stmt = stmt
}

//对应prepare函数的参数执行 返回最后的id
func (dd *DB) IntsertData(args ...interface{}) int64 {
	res, err := dd.stmt.Exec(args[0].(string))
	checkErr(err)
	//获取最后插入的数据id
	id, err := res.LastInsertId()
	checkErr(err)
	return id
}

//根据账号一个参数查找，获取clientID和password
func (dd *DB) SelectClient(args ...interface{}) (string, string, error) {
	clientID := ""
	password := ""
	//res, err := dd.stmt.Exec(args[0].(string),args[1].(string))
	id := args[0].(string)
	if id == "" {
		return "", "", errors.New("未获取到账号信息")
	}
	err := dd.stmt.QueryRow(args[0].(string)).Scan(&clientID, &password)
	if err != nil {
		return "", "", err
	}
	return clientID, password, nil
}
func main() {
	//dd := DB{}
	//db := dd.OpenLink("mysql","root:root@tcp(127.0.0.1:3306)/test2")//注意严格区分大小写
	//dd.Prepare(db,"insert data set name=?")
	//fmt.Println("LastId: ",dd.IntsertData("ljobiin"))
	//fmt.Println("LastId: ",dd.IntsertData("ljobiin"))
}
func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
