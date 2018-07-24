package mysqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	libmsg "github.com/huangjunwen/platform-kit/msg"
)

// MySQLMsgStore 实现 libmsg.MsgStore 接口; 它会在指定 db 中建立一个表 (CREATE IF NOT EXISTS)，
// 业务事务中可往该表写入需要发布的消息，然后由 MsgConnector 负责发布；
// 实现可靠的消息发布 (at least once)
type MySQLMsgStore struct {
	db          *sql.DB
	tableName   string
	selectQuery string
	insertQuery string
}

type nxMySQLMsg struct {
	id      int
	subject string
	data    []byte
}

// Queryer 抽象 sql.DB/sql.Conn/sql.Tx
type Queryer interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

var (
	_ libmsg.MsgEntry = (*nxMySQLMsg)(nil)
	_ libmsg.MsgStore = (*MySQLMsgStore)(nil)
)

// NewMySQLMsgStore 新建一个 MySQLMsgStore
func NewMySQLMsgStore(db *sql.DB, tableName string) (*MySQLMsgStore, error) {

	// 创建一个表用于存放要消息
	_, err := db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id INT NOT NULL AUTO_INCREMENT,
			subject VARCHAR(128) NOT NULL DEFAULT "",
			data BLOB,
			PRIMARY KEY (id)
		)
	`, tableName))
	if err != nil {
		return nil, err
	}

	return &MySQLMsgStore{
		db:          db,
		tableName:   tableName,
		selectQuery: fmt.Sprintf("SELECT id, subject, data FROM %s ORDER BY id", tableName),
		insertQuery: fmt.Sprintf("INSERT INTO %s (subject, data) VALUES (?, ?)", tableName),
	}, nil

}

// Fetch 实现 libmsg.MsgStore 接口
func (s *MySQLMsgStore) Fetch() <-chan libmsg.MsgEntry {
	rows, err := s.db.Query(s.selectQuery)
	if err != nil {
		// TODO: log
		return closedch
	}

	ch := make(chan libmsg.MsgEntry)
	go func() {
		defer close(ch)
		defer rows.Close()
		for rows.Next() {
			m := &nxMySQLMsg{}
			if err := rows.Scan(&m.id, &m.subject, &m.data); err != nil {
				// TODO: log
				break
			}
			ch <- m
		}
	}()

	return ch
}

// ProcessResult 实现 libmsg.MsgStore 接口
func (s *MySQLMsgStore) ProcessResult(msgs []libmsg.MsgEntry, results []bool) {
	ids := []byte{} // "1,2,3,4"
	for i, msg := range msgs {
		if !results[i] {
			continue
		}
		if len(ids) != 0 {
			// 不是第一个
			ids = append(ids, ',')
		}
		ids = append(ids, strconv.Itoa(msg.(*nxMySQLMsg).id)...)
	}

	// 全部失败了
	if len(ids) == 0 {
		return
	}

	// 删除成功发布的消息
	query := fmt.Sprintf("DELETE FROM %s WHERE id IN (%s)", s.tableName, ids)
	// TODO: err log
	s.db.Exec(query)

}

// Publish 往数据库中添加一个要发布的 Msg，主题为 subject, 数据为 data；该方法应该在事务中进行，
// 在事务成功提交后应当 kick 一下 connector，使之将刚刚添加的 Msg 发布出去
func (s *MySQLMsgStore) Publish(ctx context.Context, queryer Queryer, subject string, data []byte) error {
	_, err := queryer.ExecContext(ctx, s.insertQuery, subject, data)
	return err
}

func (m *nxMySQLMsg) Subject() string {
	return m.subject
}

func (m *nxMySQLMsg) Data() []byte {
	return m.data
}

var (
	closedch = make(chan libmsg.MsgEntry)
)

func init() {
	close(closedch)
}
