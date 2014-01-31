package command

import (
	"github.com/coreos/raft"
	//"stripe-ctf.com/sqlcluster/sql"
	"database/sql"
	"github.com/mattn/go-sqlite3"
)

type SqlQuery struct {
	Query string `json:"query"`
}

func NewSqlQuery(query string) *SqlQuery {
	return &SqlQuery{
		Query: query,
	}
}

func (q *SqlQuery) CommandName() string {
	return "query"
}

func (q *SqlQuery) Apply(context raft.Context) (interface{}, error) {
	sqlDB := context.Server().Context().(*sql.SQL)
	//output, err := sqlDB.Execute(q.Query)
	//return output, err
	db, err := sql.Open("sqlite3", "./foo.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(q.Query)
	for rows.Next() {
		var id int
		var name string
		rows.Scan(&id, &name)
		fmt.Println(id, name)
	}
	rows.Close()

}
