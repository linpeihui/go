package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/julienschmidt/httprouter"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"strconv"
)

type UserInfo struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type UserList struct {
	Users []UserInfo `json:"userList"`
}

type RelationshipState struct {
	State string `json:"state"`
}

type RelationShip struct {
	UserId int    `json:"user_id"`
	State  string `json:"state"`
	Type   string `json:"type"`
}

type RelationshipList struct {
	Relationships []RelationShip `json:"relationshipList"`
}

const (
	DISLIKED_INT = -1
	UNKNOWN_INT  = 0
	LIKED_INT    = 1

	DISLIKED_STR      = "disliked"
	MATCHED_STR       = "matched"
	LIKED_STR         = "liked"
	USER_TYPE         = "user"
	RELATIONSHIP_TYPE = "relationship"

	ERROR_RESP_MSG          = "{code:500}"
	SUCCESS_RESP_MSG_FORMAT = "{code:200, data:%s}"

	GET_USERS_SQL                       = "SELECT id, user_name FROM user_info"
	ADD_USERS_SQL                       = "INSERT INTO user_info(user_name) VALUES($1) returning id;"
	GET_RELATIONSHIPS_BY_USER_SQL       = "SELECT other_user_id, user_state, other_user_state FROM relationships WHERE user_id = $1"
	GET_RELATIONSHIPS_BY_OTHER_USER_SQL = "SELECT user_id, other_user_state, user_state FROM relationships WHERE other_user_id = $1"
	ADD_RELATIONSHIPS_BY_USER_SQL       = "INSERT INTO relationships(user_id, other_user_id, user_state) VALUES($1, $2, $3)" +
		"ON CONFLICT (user_id, other_user_id) DO UPDATE SET user_state = $3 returning id;"
	ADD_RELATIONSHIPS_BY_OTHER_USER_SQL = "INSERT INTO relationships(user_id, other_user_id, other_user_state) VALUES($1, $2, $3)" +
		"ON CONFLICT (user_id, other_user_id) DO UPDATE SET other_user_state = $3 returning id;"
)

var db *sql.DB

/**
 * 获取所有用户
 */
func getusers(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	defer recovery()
	var u UserList
	//查询数据
	rows, err := db.Query(GET_USERS_SQL)
	rows.Close()
	if err != nil {
		log.Errorf("checkErr. err: %v", err)
		fmt.Fprintf(w, ERROR_RESP_MSG)
		return
	}
	for rows.Next() {
		var id string
		var name string
		err = rows.Scan(&id, &name)
		if err != nil {
			log.Errorf("checkErr. err: %v", err)
			fmt.Fprintf(w, ERROR_RESP_MSG)
			return
		}
		u.Users = append(u.Users, UserInfo{Id: id, Name: name, Type: "user"})
	}
	//转换成json格式
	b, err := json.Marshal(u)
	if err != nil {
		log.Errorf("getusers json err: %v", err)
		return
	}
	fmt.Fprintf(w, SUCCESS_RESP_MSG_FORMAT, string(b))
}

/**
 * 添加新用户
 */
func adduser(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	defer recovery()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Errorf("checkErr. err: %v", err)
		fmt.Fprintf(w, ERROR_RESP_MSG)
		return
	}
	var u UserInfo
	json.Unmarshal(body, &u)
	var lastInsertId string
	err = db.QueryRow(ADD_USERS_SQL, u.Name).Scan(&lastInsertId)
	if err != nil {
		log.Errorf("checkErr. err: %v", err)
		fmt.Fprintf(w, ERROR_RESP_MSG)
		return
	}
	u.Id = lastInsertId
	u.Type = USER_TYPE
	b, err := json.Marshal(u)
	if err != nil {
		log.Errorf("checkErr. err: %v", err)
		fmt.Fprintf(w, ERROR_RESP_MSG)
		return
	}
	fmt.Fprintf(w, SUCCESS_RESP_MSG_FORMAT, string(b))
}

/**
 * 获取指定用户的关系列表
 * db中 state含义 1："liked"  -1: "disliked"
 */
func getUserRelationships(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	defer recovery()
	userId := ps.ByName("user_id")
	var relationshipList RelationshipList
	//查询数据
	rows1, err1 := db.Query(GET_RELATIONSHIPS_BY_USER_SQL, userId)
	rows2, err2 := db.Query(GET_RELATIONSHIPS_BY_OTHER_USER_SQL, userId)
	defer rows1.Close()
	defer rows2.Close()
	if err1 != nil {
		log.Errorf("checkErr. err: %v", err1)
		fmt.Fprintf(w, ERROR_RESP_MSG)
		return
	}
	if err2 != nil {
		log.Errorf("checkErr. err: %v", err2)
		fmt.Fprintf(w, ERROR_RESP_MSG)
		return
	}
	err1 = generateRelationshipList(rows1, &relationshipList)
	err2 = generateRelationshipList(rows2, &relationshipList)
	if err1 != nil {
		log.Errorf("getUserRelationships. json err: %v", err1)
		return
	}
	if err2 != nil {
		log.Errorf("getUserRelationships. json err: %v", err2)
		return
	}
	b, err := json.Marshal(relationshipList)
	if err != nil {
		log.Errorf("getUserRelationships. json err: %v", err)
		return
	}
	fmt.Fprintf(w, SUCCESS_RESP_MSG_FORMAT, string(b))
}

/**
 * 添加或者更新匹配关系
 * db中 state含义 1："liked"  -1: "disliked"
 */
func addOrUpdateRelationships(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	defer recovery()
	userId, err := strconv.Atoi(ps.ByName("user_id"))
	otherUserId, err := strconv.Atoi(ps.ByName("other_user_id"))
	//TODO 校验用户是否存在
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Errorf("checkErr. err: %v", err)
		fmt.Fprintf(w, ERROR_RESP_MSG)
		return
	}
	var s RelationshipState
	json.Unmarshal(body, &s)

	var userState = UNKNOWN_INT
	if s.State == LIKED_STR {
		userState = LIKED_INT
	} else if s.State == DISLIKED_STR {
		userState = DISLIKED_INT
	}

	var lastInsertId string
	if userId < otherUserId {
		err := db.QueryRow(ADD_RELATIONSHIPS_BY_USER_SQL, userId, otherUserId, userState).Scan(&lastInsertId)
		if err != nil {
			log.Errorf("checkErr. err: %v", err)
			fmt.Fprintf(w, ERROR_RESP_MSG)
			return
		}
	} else {
		err := db.QueryRow(ADD_RELATIONSHIPS_BY_OTHER_USER_SQL, otherUserId, userId, userState).Scan(&lastInsertId)
		if err != nil {
			log.Errorf("checkErr. err: %v", err)
			fmt.Fprintf(w, ERROR_RESP_MSG)
			return
		}
	}
	var relation = RelationShip{UserId: userId, State: s.State, Type: RELATIONSHIP_TYPE}
	b, err := json.Marshal(relation)
	if err != nil {
		log.Errorf("checkErr. err: %v", err)
		fmt.Fprintf(w, ERROR_RESP_MSG)
		return
	}
	fmt.Fprintf(w, SUCCESS_RESP_MSG_FORMAT, string(b))
}

func generateRelationshipList(rows *sql.Rows, r *RelationshipList) error {
	for rows.Next() {
		var userId int
		var userState int
		var otherUserState int
		var err = rows.Scan(&userId, &userState, &otherUserState)
		if err != nil {
			log.Errorf("checkErr. err: %v", err)
			return err
		}
		var state string
		if userState == DISLIKED_INT {
			state = DISLIKED_STR
		} else if userState == LIKED_INT && otherUserState == LIKED_INT {
			state = MATCHED_STR
		} else if userState == LIKED_INT {
			state = LIKED_STR
		}
		r.Relationships = append(r.Relationships, RelationShip{UserId: userId, State: state, Type: "relationship"})
	}
	return nil
}

func recovery() {
	if err := recover(); err != nil {
		log.Errorf("recovery. err: %v", err)
	}
}

func main() {
	var err error
	db, err = sql.Open("postgres", "user=postgres password=123456 dbname=postgres sslmode=disable")
	if err != nil {
		log.Errorf("sql.open error. %v", err)
		return
	}
	db.SetMaxOpenConns(60)
	db.SetMaxIdleConns(20)

	router := httprouter.New()
	router.GET("/users", getusers)
	router.POST("/users", adduser)
	router.GET("/users/:user_id/relationships", getUserRelationships)
	router.PUT("/users/:user_id/relationships/:other_user_id", addOrUpdateRelationships)

	log.Info(http.ListenAndServe(":9093", router))
}
