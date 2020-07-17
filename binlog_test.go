package mysql

import (
	"database/sql"
	"testing"
)
import "database/sql/driver"
import "fmt"
import _ "github.com/go-sql-driver/mysql"

const dataSource = "canal:canal@tcp(192.168.0.114:3307)/db_sync"

func OpenDB() *sql.DB {
	db, err := sql.Open("mysql", dataSource)
	if err != nil {
		panic(err)
	}
	return db
}

type MysqlConnection interface {
	DumpBinlog(serverId uint32, filename string, position uint32) (driver.Rows, error)
	RegisterSlave(serverId uint32) error
}

func Test_mysqlConn_DumpBinlog(t *testing.T) {
	db := OpenDB()
	defer db.Close()

	var filename, binlog_do_db, binlog_ignore_db, gtid string
	var position uint32

	row := db.QueryRow("SHOW MASTER STATUS")
	err := row.Scan(&filename, &position, &binlog_do_db, &binlog_ignore_db, &gtid)
	if err != nil {
		panic(err)
	}

	position = 4
	fmt.Printf("filename: %v, position: %v\n", filename, position)

	driver := db.Driver()
	conn, err := driver.Open(dataSource)
	if err != nil {
		panic(err)
	}

	serverId := uint32(12345)
	mysqlConn := conn.(MysqlConnection)
	err = mysqlConn.RegisterSlave(serverId)
	if err != nil {
		panic(err)
	}
	//filename = "mysql-bin.000836"
	//position = 4
	rows, err := mysqlConn.DumpBinlog(serverId, filename, position)
	if err != nil {
		panic(err)
	}
	if rows != nil {
		fmt.Println("Got results from binlog")
	}
}
