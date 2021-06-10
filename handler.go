package db_handler

import (
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis"
	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
	"math"
	"reflect"
	"time"
)

type DBHandler struct {
	DB        *xorm.Engine
	dbConf    *DbConfig
	redisConf *RedisConfig
	Redis     *redis.Client
}

func generateDSN(conf *DbConfig) string {
	return fmt.Sprintf("%s:%s@(%s:%d)/%s?charset=%s", conf.UserName, conf.Password, conf.Host, conf.Port, conf.DbName, conf.Charset)
}

func createDBConnection(dsn string, showSql bool) (*xorm.Engine, error) {
	db, err := xorm.NewEngine("mysql", dsn)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}

	if showSql {
		db.ShowSQL(showSql)
	}
	return db, nil
}

func New(dbConf *DbConfig, redisConf *RedisConfig) (*DBHandler, error) {
	if dbConf == nil {
		return nil, fmt.Errorf("dataBase config is nil")
	}
	dsn := generateDSN(dbConf)
	db, err := createDBConnection(dsn, dbConf.ShowLog)
	if err != nil {
		return nil, err
	}
	var client *redis.Client
	if redisConf != nil {
		client = redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", redisConf.Host, redisConf.Port),
			Password: redisConf.Password,
			DB:       redisConf.DB,
		})
		_, err = client.Ping().Result()
		if err != nil {
			return nil, err
		}
	}
	handler := &DBHandler{DB: db, dbConf: dbConf, Redis: client, redisConf: redisConf}
	return handler, nil
}

// find by id
func (db *DBHandler) Get(bean interface{}, name string, id interface{}) (bool, error) {
	refValue := reflect.ValueOf(bean)
	if refValue.Kind() != reflect.Ptr || refValue.Elem().Kind() != reflect.Struct {
		return false, fmt.Errorf("struct pointer expected")
	}
	key := fmt.Sprintf("%s|%s|%v", db.dbConf.DbName, name, id)
	// find from redis
	if db.Redis != nil {
		res, err := db.Redis.Get(key).Result()
		if err == nil {
			if res != "" {
				err = json.Unmarshal([]byte(res), bean)
				if err == nil {
					// refresh expire
					db.Redis.Set(key, res, time.Second*time.Duration(db.redisConf.Expire))
					return true, nil
				}
			}

			// del from redis
			db.Redis.Del(key)
		}
	}

	//find from db
	has, err := db.DB.Table(name).Where("id=?", id).Get(bean)
	if err == nil && has && db.Redis != nil {
		// save to redis
		r, err := json.Marshal(bean)
		if err == nil {
			db.Redis.Set(key, string(r), time.Second*time.Duration(db.redisConf.Expire))
		}
	}
	return has, err
}

func (db *DBHandler) GetOne(bean interface{}, name string, field string, value interface{}, idName ...string) (bool, error) {
	refValue := reflect.ValueOf(bean)
	if refValue.Kind() != reflect.Ptr || refValue.Elem().Kind() != reflect.Struct {
		return false, fmt.Errorf("struct pointer expected")
	}
	refValue = refValue.Elem()
	key := fmt.Sprintf("%s|%s|%s|%v", db.dbConf.DbName, name, field, value)
	//find from redis
	if db.Redis != nil {
		id, err := db.Redis.Get(key).Result()
		if err == nil {
			// 如果有id 走get方法
			if id != "" {
				has, err := db.Get(bean, name, id)
				if err == nil && has {
					return has, err
				}
			}
			// 如果没有找到id，则删除该key
			db.Redis.Del(key)
		}
	}
	// find from db
	has, err := db.DB.Table(name).Where(fmt.Sprintf("%s=?", field), value).Get(bean)
	if err == nil && has && db.Redis != nil {
		//save to redis
		defaultIdName := "Id"
		if idName != nil {
			defaultIdName = idName[0]
		}
		camelId := ToCamelString(defaultIdName)
		idValue := fmt.Sprintf("%v", refValue.FieldByName(camelId))
		db.Redis.Set(key, idValue, time.Second*time.Duration(db.redisConf.Expire))
	}
	return has, err
}
func (db *DBHandler) FindOne(bean interface{}, name string, condition *Condition) (bool, error) {
	session := db.DB.Table(name)
	if condition != nil {
		if condition.Where != "" {
			if condition.Params != nil {
				session = session.Where(condition.Where, condition.Params...)
			} else {
				session = session.Where(condition.Where)
			}
		}
		if condition.Asc != nil {
			session = session.Asc(condition.Asc...)
		}

		if condition.Desc != nil {
			session = session.Desc(condition.Desc...)
		}
	}
	return session.Get(bean)
}
func (db *DBHandler) List(bean interface{}, name string, condition *Condition) error {
	session := db.DB.Table(name)
	if condition != nil {
		if condition.Where != "" {
			if condition.Params != nil {
				session = session.Where(condition.Where, condition.Params...)
			} else {
				session = session.Where(condition.Where)
			}
		}

		if condition.Asc != nil {
			session = session.Asc(condition.Asc...)
		}

		if condition.Desc != nil {
			session = session.Desc(condition.Desc...)
		}

		if condition.Limit > 0 {
			session = session.Limit(condition.Limit, condition.Offset)
		}
	}
	return session.Find(bean)
}

func (db *DBHandler) ListAndCount(bean interface{}, name string, condition *Condition) (int64, error) {
	session := db.DB.Table(name)
	if condition != nil {
		if condition.Where != "" {
			if condition.Params != nil {
				session = session.Where(condition.Where, condition.Params...)
			} else {
				session = session.Where(condition.Where)
			}
		}

		if condition.Asc != nil {
			session = session.Asc(condition.Asc...)
		}

		if condition.Desc != nil {
			session = session.Desc(condition.Desc...)
		}

		if condition.Limit > 0 {
			session = session.Limit(condition.Limit, condition.Offset)
		}
	}
	return session.FindAndCount(bean)
}

func (db *DBHandler) Save(bean interface{}, name string, idName ...string) error {
	value := reflect.ValueOf(bean)
	if value.Kind() != reflect.Ptr || value.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("struct pointer expected")
	}
	defaultIdName := "Id"
	if idName != nil {
		defaultIdName = idName[0]
	}
	camelId := ToCamelString(defaultIdName)
	snakeId := ToSnakeString(defaultIdName)
	value = value.Elem()
	idValue := fmt.Sprintf("%v", value.FieldByName(camelId))
	var err error
	if idValue == "0" {
		//insert
		_, err = db.DB.Table(name).Insert(bean)
		if err != nil {
			return err
		}
	} else {
		//update
		_, err = db.DB.Table(name).Where(fmt.Sprintf("%s=?", snakeId), idValue).AllCols().Update(bean)
		if err != nil {
			return err
		}
	}
	idValue = fmt.Sprintf("%v", value.FieldByName(camelId))
	//save to redis
	if db.Redis != nil {
		key := fmt.Sprintf("%s|%s|%v", db.dbConf.DbName, name, idValue)
		r, err := json.Marshal(bean)
		if err == nil {
			db.Redis.Set(key, string(r), time.Second*time.Duration(db.redisConf.Expire))
		}
	}
	return err
}

// delete by id
func (db *DBHandler) Del(bean interface{}, name string, id interface{}) error {
	//delete from redis
	_, err := db.DB.Table(name).Where("id=?", id).Delete(bean)
	if db.Redis != nil {
		key := fmt.Sprintf("%s|%s|%v", db.dbConf.DbName, name, id)
		db.Redis.Del(key)
	}
	return err
}

// multiGet by column (default is id)
func (db *DBHandler) MultiGet(bean interface{}, name string, idList []interface{}, columnNames ...string) error {
	column := "id"
	if columnNames != nil {
		column = columnNames[0]
	}

	// todo find from redis

	err := db.DB.Table(name).In(column, idList...).Find(bean)
	return err
}

func (db *DBHandler) Count(name string, condition *Condition) (int64, error) {
	if condition == nil {
		return db.DB.Table(name).Count()
	}
	return db.DB.Table(name).Where(condition.Where, condition.Params...).Count()
}

func (db *DBHandler) Sum(bean interface{}, name string, columnName string, condition *Condition) (float64, error) {
	if condition == nil {
		return db.DB.Table(name).Sum(bean, columnName)
	}
	return db.DB.Table(name).Where(condition.Where, condition.Params...).Sum(bean, columnName)
}

func (db *DBHandler) SumInt(bean interface{}, name string, columnName string, condition *Condition) (int64, error) {
	if condition == nil {
		return db.DB.Table(name).SumInt(bean, columnName)
	}
	return db.DB.Table(name).Where(condition.Where, condition.Params...).SumInt(bean, columnName)
}

func (db *DBHandler) Sums(bean interface{}, name string, condition *Condition, columnNames ...string) ([]float64, error) {
	if condition == nil {
		return db.DB.Table(name).Sums(bean, columnNames...)
	}
	return db.DB.Table(name).Where(condition.Where, condition.Params...).Sums(bean, columnNames...)
}

func (db *DBHandler) SumsInt(bean interface{}, name string, condition *Condition, columnNames ...string) ([]int64, error) {
	if condition == nil {
		return db.DB.Table(name).SumsInt(bean, columnNames...)
	}
	return db.DB.Table(name).Where(condition.Where, condition.Params...).SumsInt(bean, columnNames...)
}

func (db *DBHandler) Exec(sql string) error {
	_, err := db.DB.Exec(sql)
	return err
}

func (db *DBHandler) Query(sql string) ([]map[string]interface{}, error) {
	return db.DB.QueryInterface(sql)
}

func (db *DBHandler) Flush() error {
	if db.Redis != nil {
		return db.Redis.FlushDB().Err()
	}
	return nil
}

func (db *DBHandler) Iterate(bean interface{}, name string, condition *Condition) <-chan interface{} {
	session := db.DB.Table(name)
	if condition != nil {
		session = session.Where(condition.Where, condition.Params...)
	}
	count, err := session.Count()
	c := make(chan interface{})

	if err != nil || count == 0 {
		go func() {
			defer close(c)
		}()
		return c
	}
	go func() {
		defer close(c)
		limit := 5000
		num := int(math.Ceil(float64(count) / float64(limit)))
		for i := 0; i < num; i++ {
			list := beanToBeans(bean)
			offset := i * limit
			s := db.DB.Table(name)
			if condition != nil {
				if condition.Where != "" {
					if condition.Params != nil {
						s = s.Where(condition.Where, condition.Params...)
					} else {
						s = s.Where(condition.Where)
					}
				}

				if condition.Asc != nil {
					s = s.Asc(condition.Asc...)
				}

				if condition.Desc != nil {
					s = s.Desc(condition.Desc...)
				}
			}
			err = s.Limit(limit, offset).Find(list)
			if err != nil {
				return
			}
			rv := reflect.ValueOf(list).Elem()
			ic := rv.Len()
			for ii := 0; ii < ic; ii++ {
				c <- rv.Index(ii).Addr().Interface()
			}
		}
	}()
	return c
}
