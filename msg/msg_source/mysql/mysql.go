package mysqlsource

import (
	"database/sql"
	"fmt"
	"strconv"

	libmsg "github.com/huangjunwen/platform-kit/msg"
)

// MySQLMsgSource 实现 libmsg.MsgSource 接口; 它会在指定 db 中建立一个表 (CREATE IF NOT EXISTS)，
// 业务事务中可往该表写入需要发布的消息，然后由 MsgConnector 负责发布；
// 实现可靠的消息发布 (at least once)
type MySQLMsgSource struct {
	db        *sql.DB
	tableName string
}

type nxMySQLMsg struct {
	id      int
	subject string
	data    []byte
}

var (
	_ libmsg.MsgEntry  = (*nxMySQLMsg)(nil)
	_ libmsg.MsgSource = (*MySQLMsgSource)(nil)
)

// NewMySQLMsgSource 新建一个 MySQLMsgSource
func NewMySQLMsgSource(db *sql.DB, tableName string) (*MySQLMsgSource, error) {

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

	return &MySQLMsgSource{
		db:        db,
		tableName: tableName,
	}, nil

}

// Fetch 实现 libmsg.MsgSource 接口
func (s *MySQLMsgSource) Fetch() <-chan libmsg.MsgEntry {
	rows, err := s.db.Query(fmt.Sprintf("SELECT id, subject, data FROM %s ORDER BY id", s.tableName))
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

// ProcessResult 实现 libmsg.MsgSource 接口
func (s *MySQLMsgSource) ProcessResult(msgs []libmsg.MsgEntry, results []bool) {
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
