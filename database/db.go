package database

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"fmt"
	"bytes"
	"strconv"
)

type Database struct {
	// connection
	conn *sql.DB
}

func (database *Database) execQuery(query string) {
	_, err := database.conn.Exec(query)

	if err != nil {
		log.Fatal(err.Error())
	}
}

func (database *Database) Connect(fileName string) error {
	db, err := sql.Open("sqlite3", fileName)
	if err != nil {
		log.Fatal(err.Error())
		return err
	}

	database.conn = db

	database.execQuery("PRAGMA foreign_keys = ON")

	database.execQuery("CREATE TABLE IF NOT EXISTS" +
		" users(id INTEGER NOT NULL PRIMARY KEY" +
		",chat_id INTEGER UNIQUE NOT NULL" +
		",is_ready INTEGER NOT NULL" +
		")")

	database.execQuery("CREATE UNIQUE INDEX IF NOT EXISTS"+
		" chat_id_index ON users(chat_id)")

	database.execQuery("CREATE TABLE IF NOT EXISTS" +
		" questions(id INTEGER NOT NULL PRIMARY KEY" +
		",author INTEGER" +
		",text STRING NOT NULL" +
		",status INTEGER NOT NULL" + // 0 - editing, 1 - opened, 2 - closed
		",end_time INTEGER NOT NULL" +
		",min_votes INTEGER NOT NULL" +
		",max_votes INTEGER NOT NULL" +
		",FOREIGN KEY(author) REFERENCES users(id) ON DELETE SET NULL" +
		")")

	database.execQuery("CREATE TABLE IF NOT EXISTS" +
		" variants(id INTEGER NOT NULL PRIMARY KEY" +
		",question_id INTEGER NOT NULL" +
		",text STRING NOT NULL" +
		",votes_count INTEGER NOT NULL" +
		",index_number INTEGER NOT NULL" +
		",FOREIGN KEY(question_id) REFERENCES questions(id) ON DELETE CASCADE" +
		")")

	database.execQuery("CREATE TABLE IF NOT EXISTS" +
		" answered_questions(id INTEGER NOT NULL PRIMARY KEY" +
		",user_id INTEGER NOT NULL" +
		",question_id INTEGER NOT NULL" +
		",FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE" +
		",FOREIGN KEY(question_id) REFERENCES questions(id) ON DELETE CASCADE" +
		")")

	database.execQuery("CREATE TABLE IF NOT EXISTS" +
		" pending_questions(id INTEGER NOT NULL PRIMARY KEY" +
		",user_id INTEGER NOT NULL" +
		",question_id INTEGER NOT NULL" +
		",FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE" +
		",FOREIGN KEY(question_id) REFERENCES questions(id) ON DELETE CASCADE" +
		")")

	return nil
}

func (database *Database) Disconnect() {
	database.conn.Close()
	database.conn = nil
}

func (database *Database) IsConnectionOpened() bool {
	return database.conn != nil
}

func (database *Database) createUniqueRecord(table string, values string) int64 {
	var err error
	if len(values) == 0 {
		_, err = database.conn.Exec(fmt.Sprintf("INSERT INTO %s DEFAULT VALUES ", table))
	} else {
		_, err = database.conn.Exec(fmt.Sprintf("INSERT INTO %s VALUES (%s)", table, values))
	}

	if err != nil {
		log.Fatal(err.Error())
		return -1
	}

	rows, err := database.conn.Query(fmt.Sprintf("SELECT id FROM %s ORDER BY id DESC LIMIT 1", table))

	if err != nil {
		log.Fatal(err.Error())
		return -1
	}
	defer rows.Close()

	if rows.Next() {
		var id int64
		err := rows.Scan(&id)
		if err != nil {
			log.Fatal(err.Error())
			return -1
		}

		return id
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal("No record created")
	return -1
}

func (database *Database) GetUserId(chatId int64) (userId int64) {
	database.execQuery(fmt.Sprintf("INSERT OR IGNORE INTO users(chat_id, is_ready) "+
		"VALUES (%d, 1)", chatId))

	rows, err := database.conn.Query(fmt.Sprintf("SELECT id FROM users WHERE chat_id=%d", chatId))
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	defer rows.Close()

	if rows.Next() {
		err := rows.Scan(&userId)
		if err != nil {
			log.Fatal(err.Error())
		}
	} else {
		err = rows.Err()
		if err != nil {
			log.Fatal(err)
		}
		log.Fatal("No user found")
	}

	return
}

func (database *Database) StartCreatingQuestion(author int64, text string, time int64, minVotes int64, maxVotes int64) {
	database.execQuery(fmt.Sprintf("UPDATE OR ROLLBACK users SET is_ready=0 WHERE id=%d", author))
	database.createUniqueRecord("questions", fmt.Sprintf("NULL,%d,'%s',0,%d,%d,%d", author, text, time, minVotes, maxVotes))
}

func (database *Database) SetVariants(questionId int64, variants []string) {
	// delete the old variants
	database.execQuery(fmt.Sprintf("DELETE FROM variants WHERE question_id=%d", questionId))

	// add the new ones
	var buffer bytes.Buffer
	count := len(variants)
	if count > 0 {
		for i, variant := range(variants) {
			buffer.WriteString(fmt.Sprintf("(%d,'%s',0,%d)", questionId, variant, i))
			if i < count - 1 {
				buffer.WriteString(",")
			}
		}

		query := fmt.Sprintf("INSERT INTO variants (question_id, text, votes_count, index_number) VALUES %s", buffer.String())
	database.execQuery(query)
	}
}

func (database *Database) EditQuestionText(questionId int64, text string) {
	database.execQuery(fmt.Sprintf("UPDATE OR ROLLBACK questions SET" +
		" text='%s'" +
		" WHERE id=%d", text, questionId))
}

func (database *Database) EditQuestionCloseRules(userId int64, time int64, minVotes int64, maxVotes int64) {
	database.execQuery(fmt.Sprintf("UPDATE OR ROLLBACK questions SET" +
		" end_time=%d" +
		",min_votes=%d" +
		",max_votes=%d" +
		" WHERE author=%d", time, minVotes, maxVotes, userId))
}

func (database *Database) CommitQuestion(userId int64) {
	// add to pending questions for all users
	database.conn.Exec(fmt.Sprintf("INSERT INTO pending_questions (user_id, question_id) " +
		"SELECT DISTINCT user_id, %d FROM users;", userId))
}

func (database *Database) DiscardQuestion(userId int64) {

}

func (database *Database) GetReadyUsersChatIds() (users []int64) {
	rows, err := database.conn.Query("SELECT chat_id FROM users WHERE is_ready=1")
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	defer rows.Close()

	for rows.Next() {
		var chatId int64
		err := rows.Scan(&chatId)
		if err != nil {
			log.Fatal(err.Error())
		}
		users = append(users, chatId)
	}

	return
}

func (database *Database) SetUserReady(userId int64) {
	database.execQuery(fmt.Sprintf("UPDATE OR ROLLBACK users SET is_ready=1 WHERE id=%d", userId))
}

func (database *Database) SetUsersUnready(chatIds []int64) {
	count := len(chatIds)
	if count > 0 {
		var buffer bytes.Buffer
		for i, chatId := range(chatIds) {
			buffer.WriteString(strconv.FormatInt(chatId, 10))
			if i < count - 1 {
				buffer.WriteString(",")
			}
		}

		database.execQuery(fmt.Sprintf("UPDATE OR ROLLBACK users SET is_ready=0 WHERE chat_id IN (%s)", buffer.String()))
	}
}

func (database *Database) GetNextQuestionForUser(userId int64) (text string, variants []string) {
	return
}

func (database *Database) AnswerNextQuestion(userId int64, index int) (hasNextQuestion bool, qusetionIsEnded bool) {
	return
}

func (database *Database) GetQuestionResults() {

}
