package orm

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
	csLog "web/csgo/log"
)

type CsDb struct {
	db     *sql.DB
	logger *csLog.Logger
	Prefix string
}

type CsSession struct {
	db        *CsDb
	tableName string
	//字段名
	fieldName []string
	//占位符
	placeHolder []string
	//修改值
	values []any
	//修改参数
	updateParam strings.Builder
	//限定参数
	whereParam strings.Builder
	//限定值
	whereValues []any
	//事务
	tx *sql.Tx
	//是否开启事务
	beginTx bool
}

func Open(driverName string, source string) *CsDb {
	//开始连接
	db, err := sql.Open(driverName, source)
	if err != nil {
		panic(err)
	}
	//最大空闲连接数，默认不配置，是2个最大空闲连接
	db.SetMaxIdleConns(5)
	//最大连接数，默认不配置，是不限制最大连接数
	db.SetMaxOpenConns(100)
	// 连接最大存活时间
	db.SetConnMaxLifetime(time.Minute * 3)
	//空闲连接最大存活时间
	db.SetConnMaxIdleTime(time.Minute * 1)
	csDb := &CsDb{
		db:     db,
		logger: csLog.Default(),
	}
	//测试连接
	err = db.Ping()
	if err != nil {
		panic(err)
	}
	return csDb
}

func (db *CsDb) SetMaxIdleConns(n int) {
	db.db.SetMaxIdleConns(n)
}

// Close 关闭连接
func (db *CsDb) Close() error {
	return db.db.Close()
}

func (db *CsDb) New(data any) *CsSession {
	m := &CsSession{
		db: db,
	}
	//对于每一次回话session都是对表进行操作，通过数据对象反射后得到表名，并绑定表名
	t := reflect.TypeOf(data)
	if t.Kind() != reflect.Pointer {

	}
	tVar := t.Elem()
	if m.tableName == "" {
		m.tableName = m.db.Prefix + strings.ToLower(Name(tVar.Name()))
	}

	return m
}

// Begin 开启事务
func (s *CsSession) Begin() error {
	tx, err := s.db.db.Begin()
	if err != nil {
		return err
	}
	s.tx = tx
	s.beginTx = true

	return nil
}

// Commit 提交事务
func (s *CsSession) Commit() error {
	err := s.tx.Commit()
	if err != nil {
		return err
	}
	s.beginTx = false
	return nil
}

// Rollback 回滚
func (s *CsSession) Rollback() error {
	err := s.tx.Rollback()
	if err != nil {
		return err
	}
	s.beginTx = false
	return nil
}

// Table 设置回话所操作的表名
func (s *CsSession) Table(name string) *CsSession {
	s.tableName = name
	return s
}

func (s *CsSession) Insert(data any) (int64, int64, error) {
	//希望每次操作都是mvcc的,Session的
	//将数据写到session中
	s.fieldNames(data)

	//拼接sql
	//sql : insert into tableName (xxx,xxx...) values(?,?...)
	query := fmt.Sprintf("insert into %s (%s) values (%s)", s.tableName, strings.Join(s.fieldName, ","), strings.Join(s.placeHolder, ","))
	s.db.logger.Info(query)
	var stmt *sql.Stmt
	var err error
	if s.beginTx {
		stmt, err = s.tx.Prepare(query)
	} else {
		//预编译出sql
		stmt, err = s.db.db.Prepare(query)
	}

	if err != nil {
		return -1, -1, err
	}
	//执行sql
	r, err := stmt.Exec(s.values...)
	if err != nil {
		return -1, -1, err
	}
	id, err := r.LastInsertId()
	if err != nil {
		return -1, -1, err
	}
	affected, err := r.RowsAffected()
	if err != nil {
		return -1, -1, err
	}

	//返回插入的ID和受影响的行数
	return id, affected, nil
}

func (s *CsSession) Update(data ...any) (int64, int64, error) {
	if len(data) == 0 || len(data) > 2 {
		return -1, -1, errors.New("param not valid")
	}
	single := true
	if len(data) == 2 {
		single = false
	}
	//Update("age",1)
	//update tableName set age = xxx,name = xxx where id = ?
	if !single {
		if s.updateParam.String() != "" {
			s.updateParam.WriteString(",")
		}
		s.updateParam.WriteString(data[0].(string))
		s.updateParam.WriteString("= ? ")
		s.values = append(s.values, data[1])
	} else {
		//Update(user)
		//update tableName set age = xxx, username = xxx ,password = ? where id = ?
		updateData := data[0]
		//得到类型
		t := reflect.TypeOf(updateData)
		//得到数据
		v := reflect.ValueOf(updateData)

		if t.Kind() != reflect.Pointer {
			panic(errors.New("updateData have to be pointer"))
		}
		tVar := t.Elem()
		vVar := v.Elem()
		if s.tableName == "" {
			s.tableName = s.db.Prefix + strings.ToLower(Name(tVar.Name()))
		}
		for i := 0; i < tVar.NumField(); i++ {
			//得到属性名
			fieldName := tVar.Field(i).Name
			//得到属性上对应的tag
			tag := tVar.Field(i).Tag
			sqlTag := tag.Get("csorm")
			if sqlTag == "" {
				//若是没有添加tag，那么自动用属性名进行转换
				sqlTag = strings.ToLower(Name(fieldName))
			} else {
				if strings.Contains(sqlTag, "auto_increment") {
					//自增字段,那么不对此属性进行执行
					continue
				}
				//如果包含逗号隔开，那么我们选第一个作为sql字段
				if strings.Contains(sqlTag, ",") {
					sqlTag = sqlTag[:strings.Index(sqlTag, ",")]
				}

			}
			//如果是自增主键
			id := vVar.Field(i).Interface()
			if strings.ToLower(sqlTag) == "id" && IsAutoId(id) {
				continue
			}
			if s.updateParam.String() != "" {
				s.updateParam.WriteString(",")
			}

			s.updateParam.WriteString(sqlTag)
			s.updateParam.WriteString("= ? ")

			//将属性对应的值填入
			s.values = append(s.values, vVar.Field(i).Interface())
		}
	}
	query := fmt.Sprintf("update %s set %s", s.tableName, s.updateParam.String())
	var sb strings.Builder
	sb.WriteString(query)
	sb.WriteString(s.whereParam.String())
	s.values = append(s.values, s.whereValues...)

	s.db.logger.Info(sb.String())
	var stmt *sql.Stmt
	var err error
	if s.beginTx {
		stmt, err = s.tx.Prepare(sb.String())
	} else {
		//预编译出sql
		stmt, err = s.db.db.Prepare(sb.String())
	}
	//预编译出sql

	if err != nil {
		return -1, -1, err
	}
	//执行sql
	r, err := stmt.Exec(s.values...)
	if err != nil {
		return -1, -1, err
	}
	id, err := r.LastInsertId()
	if err != nil {
		return -1, -1, err
	}
	affected, err := r.RowsAffected()
	if err != nil {
		return -1, -1, err
	}
	//返回插入的ID和受影响的行数
	return id, affected, nil
}

//select * from table where id = 1000

func (s *CsSession) SelectOne(data any, fields ...string) error {
	t := reflect.TypeOf(data)
	if t.Kind() != reflect.Pointer {
		return errors.New("data must be point")
	}
	fieldStr := "*"
	if len(fields) > 0 {
		fieldStr = strings.Join(fields, ",")
	}
	query := fmt.Sprintf("select %s from %s", fieldStr, s.tableName)
	var sb strings.Builder
	sb.WriteString(query)
	sb.WriteString(s.whereParam.String())

	s.db.logger.Info(sb.String())

	stmt, err := s.db.db.Prepare(sb.String())
	if err != nil {
		return err
	}
	//执行查询条件 得到结果集rows
	rows, err := stmt.Query(s.whereValues...)
	if err != nil {
		return err
	}
	//查出每行的字段
	columns, err := rows.Columns()
	if err != nil {
		return err
	}
	//存放查询数据的空对象
	values := make([]any, len(columns))
	fieldScan := make([]any, len(columns))
	//将存储地址和fieldScan绑定
	for i := range fieldScan {
		fieldScan[i] = &values[i]
	}
	if rows.Next() {
		err := rows.Scan(fieldScan...)
		if err != nil {
			return err
		}
		//得到t的数据类型（struct）
		tVar := t.Elem()
		vVar := reflect.ValueOf(data).Elem()
		for i := 0; i < tVar.NumField(); i++ {
			name := tVar.Field(i).Name
			tag := tVar.Field(i).Tag
			sqlTag := tag.Get("csorm")
			if sqlTag == "" {
				//将属性名转化为下划线名，且都为小写
				sqlTag = strings.ToLower(Name(name))
			} else {
				//如果标签有多个，那么用第一个
				if strings.Contains(sqlTag, ",") {
					sqlTag = sqlTag[:strings.Index(sqlTag, ",")]
				}
			}
			for j, colName := range columns {
				if sqlTag == colName {
					target := values[j]
					targetValue := reflect.ValueOf(target)
					fieldType := tVar.Field(i).Type
					//将目标值的类型转换成属性类型，并赋值
					result := reflect.ValueOf(targetValue.Interface()).Convert(fieldType)
					vVar.Field(i).Set(result)
				}
			}
		}
	}
	return nil
}
func (s *CsSession) Delete(data any, fields ...string) (int64, error) {
	//sql:delete from table where
	query := fmt.Sprintf("delete from %s", s.tableName)
	var sb strings.Builder
	sb.WriteString(query)
	sb.WriteString(s.whereParam.String())

	s.db.logger.Info(sb.String())

	var stmt *sql.Stmt
	var err error
	if s.beginTx {
		stmt, err = s.tx.Prepare(sb.String())
	} else {
		//预编译出sql
		stmt, err = s.db.db.Prepare(sb.String())
	}
	if err != nil {
		return -1, err
	}
	r, err := stmt.Exec(s.whereValues...)
	if err != nil {
		return -1, err
	}
	return r.RowsAffected()
}
func (s *CsSession) Select(data any, fields ...string) ([]any, error) {
	t := reflect.TypeOf(data)
	if t.Kind() != reflect.Pointer {
		return nil, errors.New("data must be point")
	}
	fieldStr := "*"
	if len(fields) > 0 {
		fieldStr = strings.Join(fields, ",")
	}
	query := fmt.Sprintf("select %s from %s", fieldStr, s.tableName)
	var sb strings.Builder
	sb.WriteString(query)
	sb.WriteString(s.whereParam.String())

	s.db.logger.Info(sb.String())

	stmt, err := s.db.db.Prepare(sb.String())
	if err != nil {
		return nil, err
	}
	//执行查询条件 得到结果集rows
	rows, err := stmt.Query(s.whereValues...)
	if err != nil {
		return nil, err
	}
	//查出每行的字段
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := make([]any, 0)
	for {
		if rows.Next() {
			//由于传进来的是一个指针地址，如果每次赋值给切片，那么都只会是一个地址
			//所以每次查询一次data就放到换一个地址？？？
			data := reflect.New(t.Elem()).Interface()
			//存放查询数据的空对象
			values := make([]any, len(columns))
			fieldScan := make([]any, len(columns))
			//将存储地址和fieldScan绑定
			for i := range fieldScan {
				fieldScan[i] = &values[i]
			}
			err := rows.Scan(fieldScan...)
			if err != nil {
				return nil, err
			}
			//得到t的数据类型（struct）
			tVar := t.Elem()
			//data是要查找的数据对象
			vVar := reflect.ValueOf(data).Elem()
			for i := 0; i < tVar.NumField(); i++ {
				name := tVar.Field(i).Name
				tag := tVar.Field(i).Tag
				sqlTag := tag.Get("csorm")
				if sqlTag == "" {
					//将属性名转化为下划线名，且都为小写
					sqlTag = strings.ToLower(Name(name))
				} else {
					//如果标签有多个，那么用第一个
					if strings.Contains(sqlTag, ",") {
						sqlTag = sqlTag[:strings.Index(sqlTag, ",")]
					}
				}
				for j, colName := range columns {
					if sqlTag == colName {
						target := values[j]
						targetValue := reflect.ValueOf(target)
						fieldType := tVar.Field(i).Type
						//将目标值的类型转换成属性类型，并赋值
						result := reflect.ValueOf(targetValue.Interface()).Convert(fieldType)
						vVar.Field(i).Set(result)
					}
				}
			}
			result = append(result, data)
		} else {
			break
		}
	}
	return result, nil
}

func (s *CsSession) Count() (int64, error) {
	return s.Aggregate("count", "*")
}
func (s *CsSession) Aggregate(funcName, field string) (int64, error) {
	var aggSb strings.Builder
	aggSb.WriteString(funcName)
	aggSb.WriteString("(")
	aggSb.WriteString(field)
	aggSb.WriteString(")")
	query := fmt.Sprintf("select %s from %s ", aggSb.String(), s.tableName)
	var sb strings.Builder
	sb.WriteString(query)
	sb.WriteString(s.whereParam.String())
	s.db.logger.Info(sb.String())
	stmt, err := s.db.db.Prepare(sb.String())
	if err != nil {
		return 0, err
	}
	var result int64
	row := stmt.QueryRow()
	err = row.Err()
	if err != nil {
		return 0, err
	}
	err = row.Scan(&result)
	if err != nil {
		return 0, err
	}
	return result, nil
}

// Exec 原生sql支持
//select * from table where id = ? =预编译=>
//select * from table where id = values ...
func (s *CsSession) Exec(query string, values ...any) (int64, error) {
	//预编译
	var stmt *sql.Stmt
	var err error
	if s.beginTx {
		stmt, err = s.tx.Prepare(query)
	} else {
		//预编译出sql
		stmt, err = s.db.db.Prepare(query)
	}

	if err != nil {
		return 0, err
	}
	result, err := stmt.Exec(values)
	if err != nil {
		return 0, err
	}
	if strings.Contains(strings.ToLower(query), "insert") {
		return result.LastInsertId()
	}
	return result.RowsAffected()
}
func (s *CsSession) QueryRow(sql string, data any, queryValues ...any) error {
	t := reflect.TypeOf(data)
	stmt, err := s.db.db.Prepare(sql)
	if err != nil {
		return err
	}
	rows, err := stmt.Query(queryValues...)
	if err != nil {
		return err
	}
	columns, err := rows.Columns()
	if err != nil {
		return err
	}
	values := make([]any, len(columns))
	var fieldsScan = make([]any, len(columns))
	for i := range fieldsScan {
		fieldsScan[i] = &values[i]
	}
	if rows.Next() {
		err := rows.Scan(fieldsScan...)
		if err != nil {
			return err
		}
		//得到t的数据类型（struct）
		tVar := t.Elem()
		vVar := reflect.ValueOf(data).Elem()
		for i := 0; i < tVar.NumField(); i++ {
			name := tVar.Field(i).Name
			tag := tVar.Field(i).Tag
			sqlTag := tag.Get("csorm")
			if sqlTag == "" {
				//将属性名转化为下划线名，且都为小写
				sqlTag = strings.ToLower(Name(name))
			} else {
				//如果标签有多个，那么用第一个
				if strings.Contains(sqlTag, ",") {
					sqlTag = sqlTag[:strings.Index(sqlTag, ",")]
				}
			}
			for j, colName := range columns {
				if sqlTag == colName {
					target := values[j]
					targetValue := reflect.ValueOf(target)
					fieldType := tVar.Field(i).Type
					//将目标值的类型转换成属性类型，并赋值
					result := reflect.ValueOf(targetValue.Interface()).Convert(fieldType)
					vVar.Field(i).Set(result)
				}
			}
		}
	}

	return nil

}

func (s *CsSession) Where(field string, value any) *CsSession {
	//id = ?
	if s.whereParam.String() == "" {
		s.whereParam.WriteString(" where ")
	}
	s.whereParam.WriteString(field)
	s.whereParam.WriteString(" = ")
	s.whereParam.WriteString(" ? ")
	s.whereValues = append(s.whereValues, value)
	return s
}
func (s *CsSession) Like(field string, value any) *CsSession {
	//name like %s%
	if s.whereParam.String() == "" {
		s.whereParam.WriteString(" where ")
	}
	s.whereParam.WriteString(field)
	s.whereParam.WriteString(" like ")
	s.whereParam.WriteString(" ? ")
	s.whereValues = append(s.whereValues, "%"+value.(string)+"%")
	return s
}
func (s *CsSession) LikeLift(field string, value any) *CsSession {
	//name like %s%
	if s.whereParam.String() == "" {
		s.whereParam.WriteString(" where ")
	}
	s.whereParam.WriteString(field)
	s.whereParam.WriteString(" like ")
	s.whereParam.WriteString(" ? ")
	s.whereValues = append(s.whereValues, "%"+value.(string))
	return s
}
func (s *CsSession) LikeRight(field string, value any) *CsSession {
	//name like %s%
	if s.whereParam.String() == "" {
		s.whereParam.WriteString(" where ")
	}
	s.whereParam.WriteString(field)
	s.whereParam.WriteString(" like ")
	s.whereParam.WriteString(" ? ")
	s.whereValues = append(s.whereValues, value.(string)+"%")
	return s
}
func (s *CsSession) Group(field ...string) *CsSession {
	//group by ff

	s.whereParam.WriteString(" group by ")
	s.whereParam.WriteString(strings.Join(field, ","))
	return s
}
func (s *CsSession) OrderDesc(field ...string) *CsSession {
	//order by aa,bb desc
	s.whereParam.WriteString(" order by ")
	s.whereParam.WriteString(strings.Join(field, ","))
	s.whereParam.WriteString(" desc ")
	return s
}
func (s *CsSession) OrderAsc(field ...string) *CsSession {
	//order by aa,bb asc
	s.whereParam.WriteString(" order by ")
	s.whereParam.WriteString(strings.Join(field, ","))
	s.whereParam.WriteString(" asc ")
	return s
}

//Order // order by name asc,age desc
func (s *CsSession) Order(field ...string) *CsSession {
	s.whereParam.WriteString(" order by ")
	size := len(field)
	if size%2 != 0 {
		panic("Order field must be 偶数")
	}
	for index, v := range field {
		s.whereParam.WriteString(" ")
		s.whereParam.WriteString(v)
		s.whereParam.WriteString(" ")
		if index%2 != 0 && index < len(field)-1 {
			s.whereParam.WriteString(",")
		}
	}
	return s
}

func (s *CsSession) And() *CsSession {
	s.whereParam.WriteString(" and ")
	return s
}

func (s *CsSession) Or() *CsSession {
	s.whereParam.WriteString(" or ")
	return s
}

//用反射获得字段名 和 值
func (s *CsSession) fieldNames(data any) {
	//得到类型
	t := reflect.TypeOf(data)
	//得到数据
	v := reflect.ValueOf(data)

	if t.Kind() != reflect.Pointer {
		panic(errors.New("data have to be pointer"))
	}
	tVar := t.Elem()
	vVar := v.Elem()
	if s.tableName == "" {
		s.tableName = s.db.Prefix + strings.ToLower(Name(tVar.Name()))
	}
	for i := 0; i < tVar.NumField(); i++ {
		//得到属性名
		fieldName := tVar.Field(i).Name
		//得到属性上对应的tag
		tag := tVar.Field(i).Tag
		sqlTag := tag.Get("csorm")
		if sqlTag == "" {
			//若是没有添加tag，那么自动用属性名进行转换
			sqlTag = strings.ToLower(Name(fieldName))
		} else {
			if strings.Contains(sqlTag, "auto_increment") {
				//自增字段,那么不对此属性进行执行
				continue
			}
			//如果包含逗号隔开，那么我们选第一个作为sql字段
			if strings.Contains(sqlTag, ",") {
				sqlTag = sqlTag[:strings.Index(sqlTag, ",")]
			}

		}
		//如果是自增主键
		id := vVar.Field(i).Interface()
		if strings.ToLower(sqlTag) == "id" && IsAutoId(id) {
			continue
		}
		s.fieldName = append(s.fieldName, sqlTag)
		s.placeHolder = append(s.placeHolder, "?")
		//将属性对应的值填入
		s.values = append(s.values, vVar.Field(i).Interface())
	}
}

func (s *CsSession) BatchInsert(data []any) (int64, int64, error) {
	if len(data) == 0 {
		return -1, -1, errors.New("no data insert")
	}
	//批量插入 insert into table (x,x) values (),()
	s.batchFieldNames(data)
	query := fmt.Sprintf("insert into %s (%s) values ", s.tableName, strings.Join(s.fieldName, ","))
	var sb strings.Builder
	sb.WriteString(query)
	for index, _ := range data {
		sb.WriteString("(")
		sb.WriteString(strings.Join(s.placeHolder, ","))
		sb.WriteString(")")
		if index < len(data)-1 {
			sb.WriteString(",")
		}
	}
	var stmt *sql.Stmt
	var err error
	if s.beginTx {
		stmt, err = s.tx.Prepare(sb.String())
	} else {
		//预编译出sql
		stmt, err = s.db.db.Prepare(sb.String())
	}

	if err != nil {
		return -1, -1, err
	}
	r, err := stmt.Exec(s.values...)
	if err != nil {
		s.db.logger.Error(err)
		return -1, -1, err
	}
	id, err := r.LastInsertId()
	if err != nil {
		s.db.logger.Error(err)
		return -1, -1, err
	}
	affected, err := r.RowsAffected()
	if err != nil {
		s.db.logger.Error(err)
		return -1, -1, err
	}
	return id, affected, nil
}

func (s *CsSession) batchFieldNames(dataArray []any) {
	data := dataArray[0]
	t := reflect.TypeOf(data)
	v := reflect.ValueOf(data)
	if t.Kind() != reflect.Pointer {
		panic(errors.New("batch insert element type must be pointer"))
	}
	tVar := t.Elem()
	vVar := v.Elem()
	if s.tableName == "" {
		s.tableName = s.db.Prefix + strings.ToLower(Name(tVar.Name()))
	}

	var fieldNames []string
	var placeholder []string
	for i := 0; i < tVar.NumField(); i++ {
		//首字母是小写的
		if !vVar.Field(i).CanInterface() {
			continue
		}
		//解析tag
		field := tVar.Field(i)
		sqlTag := field.Tag.Get("mssql")
		if sqlTag == "" {
			sqlTag = strings.ToLower(Name(field.Name))
		}
		contains := strings.Contains(sqlTag, "auto_increment")
		if sqlTag == "id" || contains {
			//对id做个判断 如果其值小于等于0 数据库可能是自增 跳过此字段
			if IsAutoId(vVar.Field(i).Interface()) {
				continue
			}
		}
		if contains {
			sqlTag = sqlTag[:strings.Index(sqlTag, ",")]
		}
		fieldNames = append(fieldNames, sqlTag)
		placeholder = append(placeholder, "?")
	}
	s.fieldName = fieldNames
	s.placeHolder = placeholder
	var allValues []any
	for _, value := range dataArray {
		t := reflect.TypeOf(value)
		v := reflect.ValueOf(value)
		tVar := t.Elem()
		vVar := v.Elem()
		for i := 0; i < tVar.NumField(); i++ {
			//首字母是小写的
			if !vVar.Field(i).CanInterface() {
				continue
			}
			//解析tag
			field := tVar.Field(i)
			sqlTag := field.Tag.Get("mssql")
			if sqlTag == "" {
				sqlTag = strings.ToLower(Name(field.Name))
			}
			contains := strings.Contains(sqlTag, "auto_increment")
			if sqlTag == "id" || contains {
				//对id做个判断 如果其值小于等于0 数据库可能是自增 跳过此字段
				if IsAutoId(vVar.Field(i).Interface()) {
					continue
				}
			}
			if contains {
				sqlTag = sqlTag[:strings.Index(sqlTag, ",")]
			}
			allValues = append(allValues, vVar.Field(i).Interface())
		}
	}
	s.values = allValues
}

func (s *CsSession) batchValues(data []any) {
	s.values = make([]any, 0)
	for _, v := range data {
		//得到类型
		t := reflect.TypeOf(v)
		//得到数据
		v := reflect.ValueOf(v)
		if t.Kind() != reflect.Pointer {
			panic(errors.New("data have to be pointer"))
		}
		tVar := t.Elem()
		vVar := v.Elem()
		if s.tableName == "" {
			s.tableName = s.db.Prefix + strings.ToLower(Name(tVar.Name()))
		}
		for i := 0; i < tVar.NumField(); i++ {
			//得到属性名
			fieldName := tVar.Field(i).Name
			//得到属性上对应的tag
			tag := tVar.Field(i).Tag
			sqlTag := tag.Get("csorm")
			if sqlTag == "" {
				//若是没有添加tag，那么自动用属性名进行转换
				sqlTag = strings.ToLower(Name(fieldName))
			} else {
				if strings.Contains(sqlTag, "auto_increment") {
					//自增字段,那么不对此属性进行执行
					continue
				}
				//如果包含逗号隔开，那么我们选第一个作为sql字段
				if strings.Contains(sqlTag, ",") {
					sqlTag = sqlTag[:strings.Index(sqlTag, ",")]
				}

			}
			//如果是自增主键
			id := vVar.Field(i).Interface()
			if strings.ToLower(sqlTag) == "id" && IsAutoId(id) {
				continue
			}
			//将属性对应的值填入
			s.values = append(s.values, vVar.Field(i).Interface())
		}
	}
}

func IsAutoId(id any) bool {
	t := reflect.TypeOf(id)
	switch t.Kind() {
	case reflect.Int64:
		if id.(int64) <= 0 {
			return true
		}
	case reflect.Int32:
		if id.(int32) <= 0 {
			return true
		}
	case reflect.Int:
		if id.(int) <= 0 {
			return true
		}
	default:
		return false
	}
	return false
}

// Name 将属性名转换为sql字段名（驼峰转下划）
func Name(name string) string {
	//得到字符串切片
	var names = name[:]
	lastIndex := 0
	var sb strings.Builder
	for index, value := range names {
		if value >= 65 && value <= 90 {
			//大写字母
			if index == 0 {
				continue
			}
			sb.WriteString(name[:index])
			sb.WriteString("_")
			lastIndex = index
		}
	}
	//if lastIndex <= len(names)-1 {
	sb.WriteString(name[lastIndex:])
	//}
	return sb.String()
}
