package autocompress

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestMysql(t *testing.T) {
	url := "root:root@(127.0.0.1:3306)/simq?charset=utf8mb4&parseTime=True&loc=Local"
	db2, _ := gorm.Open(mysql.Open(url), &gorm.Config{})
	db2.AutoMigrate(&Sub{})
	db2.Exec("delete from sub")
	go func() {
		num := 10 // 验证条件：数据库数据每个sender和topic 对应的订阅与取消订阅 的 num 相减 等于（sub-unsub）* num -1 即可
		for {
			go autoInsert(db2, 30, 20, "sender111", "a/b/c")
			go autoInsert(db2, 30, 20, "sender222", "a/b/c")
			go autoInsert(db2, 30, 20, "sender111", "a/b/c2222222")
			num--
			time.Sleep(5 * time.Second)
			if num == 0 {
				time.Sleep(5 * time.Second)
				break
			}
		}
	}()

	SubAutoCompress(url, 10, 5, NewAutoCompress("sender", url))

	select {}
}

func autoInsert(db2 *gorm.DB, sub, unsub int, sender, topic string) {
	for i := 0; i < sub; i++ {
		b := db2.Table("sub").Create(&Sub{
			SubOrUnSub: 1,
			Sender:     sender,
			Topic:      topic,
			Num:        0,
		})
		if b.Error != nil {
			println(b.Error)
		}
	}
	for i := 0; i < unsub; i++ {
		b := db2.Table("sub").Create(&Sub{
			SubOrUnSub: 2,
			Sender:     sender,
			Topic:      topic,
			Num:        0,
		})
		if b.Error != nil {
			println(b.Error)
		}
	}
}
