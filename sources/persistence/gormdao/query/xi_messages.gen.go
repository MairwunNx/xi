// Code generated by gorm.io/gen. DO NOT EDIT.
// Code generated by gorm.io/gen. DO NOT EDIT.
// Code generated by gorm.io/gen. DO NOT EDIT.

package query

import (
	"context"
	"database/sql"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"

	"gorm.io/gen"
	"gorm.io/gen/field"

	"gorm.io/plugin/dbresolver"

	"ximanager/sources/persistence/entities"
)

func newMessage(db *gorm.DB, opts ...gen.DOOption) message {
	_message := message{}

	_message.messageDo.UseDB(db, opts...)
	_message.messageDo.UseModel(&entities.Message{})

	tableName := _message.messageDo.TableName()
	_message.ALL = field.NewAsterisk(tableName)
	_message.ID = field.NewField(tableName, "id")
	_message.ChatID = field.NewInt64(tableName, "chat_id")
	_message.MessageTime = field.NewTime(tableName, "message_time")
	_message.MessageText = field.NewString(tableName, "message_text")
	_message.IsAggressive = field.NewBool(tableName, "is_aggressive")
	_message.IsXiResponse = field.NewBool(tableName, "is_xi_response")
	_message.IsRemoved = field.NewBool(tableName, "is_removed")
	_message.UserID = field.NewField(tableName, "user_id")
	_message.User = messageHasOneUser{
		db: db.Session(&gorm.Session{}),

		RelationField: field.NewRelation("User", "entities.User"),
		Messages: struct {
			field.RelationField
			User struct {
				field.RelationField
			}
		}{
			RelationField: field.NewRelation("User.Messages", "entities.Message"),
			User: struct {
				field.RelationField
			}{
				RelationField: field.NewRelation("User.Messages.User", "entities.User"),
			},
		},
		Donations: struct {
			field.RelationField
			UserEntity struct {
				field.RelationField
			}
		}{
			RelationField: field.NewRelation("User.Donations", "entities.Donation"),
			UserEntity: struct {
				field.RelationField
			}{
				RelationField: field.NewRelation("User.Donations.UserEntity", "entities.User"),
			},
		},
		CreatedModes: struct {
			field.RelationField
			Creator struct {
				field.RelationField
			}
			SelectedModes struct {
				field.RelationField
				Mode struct {
					field.RelationField
				}
				User struct {
					field.RelationField
				}
			}
		}{
			RelationField: field.NewRelation("User.CreatedModes", "entities.Mode"),
			Creator: struct {
				field.RelationField
			}{
				RelationField: field.NewRelation("User.CreatedModes.Creator", "entities.User"),
			},
			SelectedModes: struct {
				field.RelationField
				Mode struct {
					field.RelationField
				}
				User struct {
					field.RelationField
				}
			}{
				RelationField: field.NewRelation("User.CreatedModes.SelectedModes", "entities.SelectedMode"),
				Mode: struct {
					field.RelationField
				}{
					RelationField: field.NewRelation("User.CreatedModes.SelectedModes.Mode", "entities.Mode"),
				},
				User: struct {
					field.RelationField
				}{
					RelationField: field.NewRelation("User.CreatedModes.SelectedModes.User", "entities.User"),
				},
			},
		},
		SelectedModes: struct {
			field.RelationField
		}{
			RelationField: field.NewRelation("User.SelectedModes", "entities.SelectedMode"),
		},
		Pins: struct {
			field.RelationField
			UserEntity struct {
				field.RelationField
			}
		}{
			RelationField: field.NewRelation("User.Pins", "entities.Pin"),
			UserEntity: struct {
				field.RelationField
			}{
				RelationField: field.NewRelation("User.Pins.UserEntity", "entities.User"),
			},
		},
		Usages: struct {
			field.RelationField
			User struct {
				field.RelationField
			}
		}{
			RelationField: field.NewRelation("User.Usages", "entities.Usage"),
			User: struct {
				field.RelationField
			}{
				RelationField: field.NewRelation("User.Usages.User", "entities.User"),
			},
		},
	}

	_message.fillFieldMap()

	return _message
}

type message struct {
	messageDo messageDo

	ALL          field.Asterisk
	ID           field.Field
	ChatID       field.Int64
	MessageTime  field.Time
	MessageText  field.String
	IsAggressive field.Bool
	IsXiResponse field.Bool
	IsRemoved    field.Bool
	UserID       field.Field
	User         messageHasOneUser

	fieldMap map[string]field.Expr
}

func (m message) Table(newTableName string) *message {
	m.messageDo.UseTable(newTableName)
	return m.updateTableName(newTableName)
}

func (m message) As(alias string) *message {
	m.messageDo.DO = *(m.messageDo.As(alias).(*gen.DO))
	return m.updateTableName(alias)
}

func (m *message) updateTableName(table string) *message {
	m.ALL = field.NewAsterisk(table)
	m.ID = field.NewField(table, "id")
	m.ChatID = field.NewInt64(table, "chat_id")
	m.MessageTime = field.NewTime(table, "message_time")
	m.MessageText = field.NewString(table, "message_text")
	m.IsAggressive = field.NewBool(table, "is_aggressive")
	m.IsXiResponse = field.NewBool(table, "is_xi_response")
	m.IsRemoved = field.NewBool(table, "is_removed")
	m.UserID = field.NewField(table, "user_id")

	m.fillFieldMap()

	return m
}

func (m *message) WithContext(ctx context.Context) IMessageDo { return m.messageDo.WithContext(ctx) }

func (m message) TableName() string { return m.messageDo.TableName() }

func (m message) Alias() string { return m.messageDo.Alias() }

func (m message) Columns(cols ...field.Expr) gen.Columns { return m.messageDo.Columns(cols...) }

func (m *message) GetFieldByName(fieldName string) (field.OrderExpr, bool) {
	_f, ok := m.fieldMap[fieldName]
	if !ok || _f == nil {
		return nil, false
	}
	_oe, ok := _f.(field.OrderExpr)
	return _oe, ok
}

func (m *message) fillFieldMap() {
	m.fieldMap = make(map[string]field.Expr, 9)
	m.fieldMap["id"] = m.ID
	m.fieldMap["chat_id"] = m.ChatID
	m.fieldMap["message_time"] = m.MessageTime
	m.fieldMap["message_text"] = m.MessageText
	m.fieldMap["is_aggressive"] = m.IsAggressive
	m.fieldMap["is_xi_response"] = m.IsXiResponse
	m.fieldMap["is_removed"] = m.IsRemoved
	m.fieldMap["user_id"] = m.UserID

}

func (m message) clone(db *gorm.DB) message {
	m.messageDo.ReplaceConnPool(db.Statement.ConnPool)
	m.User.db = db.Session(&gorm.Session{Initialized: true})
	m.User.db.Statement.ConnPool = db.Statement.ConnPool
	return m
}

func (m message) replaceDB(db *gorm.DB) message {
	m.messageDo.ReplaceDB(db)
	m.User.db = db.Session(&gorm.Session{})
	return m
}

type messageHasOneUser struct {
	db *gorm.DB

	field.RelationField

	Messages struct {
		field.RelationField
		User struct {
			field.RelationField
		}
	}
	Donations struct {
		field.RelationField
		UserEntity struct {
			field.RelationField
		}
	}
	CreatedModes struct {
		field.RelationField
		Creator struct {
			field.RelationField
		}
		SelectedModes struct {
			field.RelationField
			Mode struct {
				field.RelationField
			}
			User struct {
				field.RelationField
			}
		}
	}
	SelectedModes struct {
		field.RelationField
	}
	Pins struct {
		field.RelationField
		UserEntity struct {
			field.RelationField
		}
	}
	Usages struct {
		field.RelationField
		User struct {
			field.RelationField
		}
	}
}

func (a messageHasOneUser) Where(conds ...field.Expr) *messageHasOneUser {
	if len(conds) == 0 {
		return &a
	}

	exprs := make([]clause.Expression, 0, len(conds))
	for _, cond := range conds {
		exprs = append(exprs, cond.BeCond().(clause.Expression))
	}
	a.db = a.db.Clauses(clause.Where{Exprs: exprs})
	return &a
}

func (a messageHasOneUser) WithContext(ctx context.Context) *messageHasOneUser {
	a.db = a.db.WithContext(ctx)
	return &a
}

func (a messageHasOneUser) Session(session *gorm.Session) *messageHasOneUser {
	a.db = a.db.Session(session)
	return &a
}

func (a messageHasOneUser) Model(m *entities.Message) *messageHasOneUserTx {
	return &messageHasOneUserTx{a.db.Model(m).Association(a.Name())}
}

func (a messageHasOneUser) Unscoped() *messageHasOneUser {
	a.db = a.db.Unscoped()
	return &a
}

type messageHasOneUserTx struct{ tx *gorm.Association }

func (a messageHasOneUserTx) Find() (result *entities.User, err error) {
	return result, a.tx.Find(&result)
}

func (a messageHasOneUserTx) Append(values ...*entities.User) (err error) {
	targetValues := make([]interface{}, len(values))
	for i, v := range values {
		targetValues[i] = v
	}
	return a.tx.Append(targetValues...)
}

func (a messageHasOneUserTx) Replace(values ...*entities.User) (err error) {
	targetValues := make([]interface{}, len(values))
	for i, v := range values {
		targetValues[i] = v
	}
	return a.tx.Replace(targetValues...)
}

func (a messageHasOneUserTx) Delete(values ...*entities.User) (err error) {
	targetValues := make([]interface{}, len(values))
	for i, v := range values {
		targetValues[i] = v
	}
	return a.tx.Delete(targetValues...)
}

func (a messageHasOneUserTx) Clear() error {
	return a.tx.Clear()
}

func (a messageHasOneUserTx) Count() int64 {
	return a.tx.Count()
}

func (a messageHasOneUserTx) Unscoped() *messageHasOneUserTx {
	a.tx = a.tx.Unscoped()
	return &a
}

type messageDo struct{ gen.DO }

type IMessageDo interface {
	gen.SubQuery
	Debug() IMessageDo
	WithContext(ctx context.Context) IMessageDo
	WithResult(fc func(tx gen.Dao)) gen.ResultInfo
	ReplaceDB(db *gorm.DB)
	ReadDB() IMessageDo
	WriteDB() IMessageDo
	As(alias string) gen.Dao
	Session(config *gorm.Session) IMessageDo
	Columns(cols ...field.Expr) gen.Columns
	Clauses(conds ...clause.Expression) IMessageDo
	Not(conds ...gen.Condition) IMessageDo
	Or(conds ...gen.Condition) IMessageDo
	Select(conds ...field.Expr) IMessageDo
	Where(conds ...gen.Condition) IMessageDo
	Order(conds ...field.Expr) IMessageDo
	Distinct(cols ...field.Expr) IMessageDo
	Omit(cols ...field.Expr) IMessageDo
	Join(table schema.Tabler, on ...field.Expr) IMessageDo
	LeftJoin(table schema.Tabler, on ...field.Expr) IMessageDo
	RightJoin(table schema.Tabler, on ...field.Expr) IMessageDo
	Group(cols ...field.Expr) IMessageDo
	Having(conds ...gen.Condition) IMessageDo
	Limit(limit int) IMessageDo
	Offset(offset int) IMessageDo
	Count() (count int64, err error)
	Scopes(funcs ...func(gen.Dao) gen.Dao) IMessageDo
	Unscoped() IMessageDo
	Create(values ...*entities.Message) error
	CreateInBatches(values []*entities.Message, batchSize int) error
	Save(values ...*entities.Message) error
	First() (*entities.Message, error)
	Take() (*entities.Message, error)
	Last() (*entities.Message, error)
	Find() ([]*entities.Message, error)
	FindInBatch(batchSize int, fc func(tx gen.Dao, batch int) error) (results []*entities.Message, err error)
	FindInBatches(result *[]*entities.Message, batchSize int, fc func(tx gen.Dao, batch int) error) error
	Pluck(column field.Expr, dest interface{}) error
	Delete(...*entities.Message) (info gen.ResultInfo, err error)
	Update(column field.Expr, value interface{}) (info gen.ResultInfo, err error)
	UpdateSimple(columns ...field.AssignExpr) (info gen.ResultInfo, err error)
	Updates(value interface{}) (info gen.ResultInfo, err error)
	UpdateColumn(column field.Expr, value interface{}) (info gen.ResultInfo, err error)
	UpdateColumnSimple(columns ...field.AssignExpr) (info gen.ResultInfo, err error)
	UpdateColumns(value interface{}) (info gen.ResultInfo, err error)
	UpdateFrom(q gen.SubQuery) gen.Dao
	Attrs(attrs ...field.AssignExpr) IMessageDo
	Assign(attrs ...field.AssignExpr) IMessageDo
	Joins(fields ...field.RelationField) IMessageDo
	Preload(fields ...field.RelationField) IMessageDo
	FirstOrInit() (*entities.Message, error)
	FirstOrCreate() (*entities.Message, error)
	FindByPage(offset int, limit int) (result []*entities.Message, count int64, err error)
	ScanByPage(result interface{}, offset int, limit int) (count int64, err error)
	Rows() (*sql.Rows, error)
	Row() *sql.Row
	Scan(result interface{}) (err error)
	Returning(value interface{}, columns ...string) IMessageDo
	UnderlyingDB() *gorm.DB
	schema.Tabler
}

func (m messageDo) Debug() IMessageDo {
	return m.withDO(m.DO.Debug())
}

func (m messageDo) WithContext(ctx context.Context) IMessageDo {
	return m.withDO(m.DO.WithContext(ctx))
}

func (m messageDo) ReadDB() IMessageDo {
	return m.Clauses(dbresolver.Read)
}

func (m messageDo) WriteDB() IMessageDo {
	return m.Clauses(dbresolver.Write)
}

func (m messageDo) Session(config *gorm.Session) IMessageDo {
	return m.withDO(m.DO.Session(config))
}

func (m messageDo) Clauses(conds ...clause.Expression) IMessageDo {
	return m.withDO(m.DO.Clauses(conds...))
}

func (m messageDo) Returning(value interface{}, columns ...string) IMessageDo {
	return m.withDO(m.DO.Returning(value, columns...))
}

func (m messageDo) Not(conds ...gen.Condition) IMessageDo {
	return m.withDO(m.DO.Not(conds...))
}

func (m messageDo) Or(conds ...gen.Condition) IMessageDo {
	return m.withDO(m.DO.Or(conds...))
}

func (m messageDo) Select(conds ...field.Expr) IMessageDo {
	return m.withDO(m.DO.Select(conds...))
}

func (m messageDo) Where(conds ...gen.Condition) IMessageDo {
	return m.withDO(m.DO.Where(conds...))
}

func (m messageDo) Order(conds ...field.Expr) IMessageDo {
	return m.withDO(m.DO.Order(conds...))
}

func (m messageDo) Distinct(cols ...field.Expr) IMessageDo {
	return m.withDO(m.DO.Distinct(cols...))
}

func (m messageDo) Omit(cols ...field.Expr) IMessageDo {
	return m.withDO(m.DO.Omit(cols...))
}

func (m messageDo) Join(table schema.Tabler, on ...field.Expr) IMessageDo {
	return m.withDO(m.DO.Join(table, on...))
}

func (m messageDo) LeftJoin(table schema.Tabler, on ...field.Expr) IMessageDo {
	return m.withDO(m.DO.LeftJoin(table, on...))
}

func (m messageDo) RightJoin(table schema.Tabler, on ...field.Expr) IMessageDo {
	return m.withDO(m.DO.RightJoin(table, on...))
}

func (m messageDo) Group(cols ...field.Expr) IMessageDo {
	return m.withDO(m.DO.Group(cols...))
}

func (m messageDo) Having(conds ...gen.Condition) IMessageDo {
	return m.withDO(m.DO.Having(conds...))
}

func (m messageDo) Limit(limit int) IMessageDo {
	return m.withDO(m.DO.Limit(limit))
}

func (m messageDo) Offset(offset int) IMessageDo {
	return m.withDO(m.DO.Offset(offset))
}

func (m messageDo) Scopes(funcs ...func(gen.Dao) gen.Dao) IMessageDo {
	return m.withDO(m.DO.Scopes(funcs...))
}

func (m messageDo) Unscoped() IMessageDo {
	return m.withDO(m.DO.Unscoped())
}

func (m messageDo) Create(values ...*entities.Message) error {
	if len(values) == 0 {
		return nil
	}
	return m.DO.Create(values)
}

func (m messageDo) CreateInBatches(values []*entities.Message, batchSize int) error {
	return m.DO.CreateInBatches(values, batchSize)
}

// Save : !!! underlying implementation is different with GORM
// The method is equivalent to executing the statement: db.Clauses(clause.OnConflict{UpdateAll: true}).Create(values)
func (m messageDo) Save(values ...*entities.Message) error {
	if len(values) == 0 {
		return nil
	}
	return m.DO.Save(values)
}

func (m messageDo) First() (*entities.Message, error) {
	if result, err := m.DO.First(); err != nil {
		return nil, err
	} else {
		return result.(*entities.Message), nil
	}
}

func (m messageDo) Take() (*entities.Message, error) {
	if result, err := m.DO.Take(); err != nil {
		return nil, err
	} else {
		return result.(*entities.Message), nil
	}
}

func (m messageDo) Last() (*entities.Message, error) {
	if result, err := m.DO.Last(); err != nil {
		return nil, err
	} else {
		return result.(*entities.Message), nil
	}
}

func (m messageDo) Find() ([]*entities.Message, error) {
	result, err := m.DO.Find()
	return result.([]*entities.Message), err
}

func (m messageDo) FindInBatch(batchSize int, fc func(tx gen.Dao, batch int) error) (results []*entities.Message, err error) {
	buf := make([]*entities.Message, 0, batchSize)
	err = m.DO.FindInBatches(&buf, batchSize, func(tx gen.Dao, batch int) error {
		defer func() { results = append(results, buf...) }()
		return fc(tx, batch)
	})
	return results, err
}

func (m messageDo) FindInBatches(result *[]*entities.Message, batchSize int, fc func(tx gen.Dao, batch int) error) error {
	return m.DO.FindInBatches(result, batchSize, fc)
}

func (m messageDo) Attrs(attrs ...field.AssignExpr) IMessageDo {
	return m.withDO(m.DO.Attrs(attrs...))
}

func (m messageDo) Assign(attrs ...field.AssignExpr) IMessageDo {
	return m.withDO(m.DO.Assign(attrs...))
}

func (m messageDo) Joins(fields ...field.RelationField) IMessageDo {
	for _, _f := range fields {
		m = *m.withDO(m.DO.Joins(_f))
	}
	return &m
}

func (m messageDo) Preload(fields ...field.RelationField) IMessageDo {
	for _, _f := range fields {
		m = *m.withDO(m.DO.Preload(_f))
	}
	return &m
}

func (m messageDo) FirstOrInit() (*entities.Message, error) {
	if result, err := m.DO.FirstOrInit(); err != nil {
		return nil, err
	} else {
		return result.(*entities.Message), nil
	}
}

func (m messageDo) FirstOrCreate() (*entities.Message, error) {
	if result, err := m.DO.FirstOrCreate(); err != nil {
		return nil, err
	} else {
		return result.(*entities.Message), nil
	}
}

func (m messageDo) FindByPage(offset int, limit int) (result []*entities.Message, count int64, err error) {
	result, err = m.Offset(offset).Limit(limit).Find()
	if err != nil {
		return
	}

	if size := len(result); 0 < limit && 0 < size && size < limit {
		count = int64(size + offset)
		return
	}

	count, err = m.Offset(-1).Limit(-1).Count()
	return
}

func (m messageDo) ScanByPage(result interface{}, offset int, limit int) (count int64, err error) {
	count, err = m.Count()
	if err != nil {
		return
	}

	err = m.Offset(offset).Limit(limit).Scan(result)
	return
}

func (m messageDo) Scan(result interface{}) (err error) {
	return m.DO.Scan(result)
}

func (m messageDo) Delete(models ...*entities.Message) (result gen.ResultInfo, err error) {
	return m.DO.Delete(models)
}

func (m *messageDo) withDO(do gen.Dao) *messageDo {
	m.DO = *do.(*gen.DO)
	return m
}
