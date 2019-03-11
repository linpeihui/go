package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/julienschmidt/httprouter"
	_ "github.com/lib/pq"
	"io/ioutil"
	"log"
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
	DISLIKED = -1
	UNKNOWN  = 0
	LIKED    = 1
	USER_TYPE = "user"
	RELATIONSHIP_TYPE  = "relationship"
)

var DB *sql.DB

/**
 * 获取所有用户
 */
func getusers(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	recovery()
	var u UserList
	//查询数据
	rows, err := DB.Query("SELECT id, user_name FROM user_info")
	checkErr(err)
	defer rows.Close()
	for rows.Next() {
		var id string
		var name string
		err = rows.Scan(&id, &name)
		checkErr(err)
		u.Users = append(u.Users, UserInfo{Id: id, Name: name, Type: "user"})
	}
	//转换成json格式
	b, err := json.Marshal(u)
	if err != nil {
		log.Fatalf("json err:", err)
	}
	fmt.Fprintf(w, "%s", string(b))
}

/**
 * 添加新用户
 */
func adduser(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	recovery()
	body, err := ioutil.ReadAll(r.Body)
	checkErr(err)
	var u UserInfo
	json.Unmarshal(body, &u)
	var lastInsertId string
	err = DB.QueryRow("INSERT INTO user_info(user_name) VALUES($1) returning id;", u.Name).Scan(&lastInsertId)
	checkErr(err)
	u.Id = lastInsertId
	u.Type = USER_TYPE
	b, err := json.Marshal(u)
	checkErr(err)
	fmt.Fprintf(w, "%s", string(b))
}

/**
 * 获取指定用户的关系列表
 * db中 state含义 1："liked"  -1: "disliked"
 */
func getUserRelationships(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	recovery()
	userId := ps.ByName("user_id")
	var relationshipList RelationshipList
	//查询数据
	rows1, err := DB.Query("SELECT other_user_id, user_state, other_user_state FROM relationships WHERE user_id = $1", userId)
	checkErr(err)
	rows1.Close()
	rows2, err := DB.Query("SELECT user_id, other_user_state, user_state FROM relationships WHERE other_user_id = $1", userId)
	checkErr(err)
	rows2.Close()
	generateRelationshipList(rows1, &relationshipList)
	generateRelationshipList(rows2, &relationshipList)

	b, err := json.Marshal(relationshipList)
	if err != nil {
		log.Fatalf("json err:", err)
	}
	fmt.Fprintf(w, "%s", string(b))
}

/**
 * 添加或者更新匹配关系
 * db中 state含义 1："liked"  -1: "disliked"
 */
func addOrUpdateRelationships(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	recovery()
	userId, err := strconv.Atoi(ps.ByName("user_id"))
	otherUserId, err := strconv.Atoi(ps.ByName("other_user_id"))
	body, err := ioutil.ReadAll(r.Body)
	checkErr(err)
	var s RelationshipState
	json.Unmarshal(body, &s)

	var userState = UNKNOWN
	if s.State == "liked" {
		userState = LIKED
	} else if s.State == "disliked" {
		userState = DISLIKED
	}

	var lastInsertId string
	if userId < otherUserId {
		err := DB.QueryRow("INSERT INTO relationships(user_id, other_user_id, user_state) VALUES($1, $2, $3)"+
			"ON CONFLICT (user_id, other_user_id) DO UPDATE SET user_state = $3 returning id;", userId, otherUserId, userState).Scan(&lastInsertId)
		checkErr(err)
	} else {
		err := DB.QueryRow("INSERT INTO relationships(user_id, other_user_id, other_user_state) VALUES($1, $2, $3)"+
			"ON CONFLICT (user_id, other_user_id) DO UPDATE SET other_user_state = $3 returning id;", otherUserId, userId, userState).Scan(&lastInsertId)
		checkErr(err)
	}
	var relation = RelationShip{UserId: userId, State: s.State, Type: RELATIONSHIP_TYPE}
	b, err := json.Marshal(relation)
	checkErr(err)
	fmt.Fprintf(w, "%s", string(b))
}

func generateRelationshipList(rows *sql.Rows, r *RelationshipList) {
	for rows.Next() {
		var userId int
		var userState int
		var otherUserState int
		var err = rows.Scan(&userId, &userState, &otherUserState)
		checkErr(err)
		var state string
		if userState == DISLIKED {
			state = "disliked"
		} else if userState == LIKED && otherUserState == LIKED {
			state = "matched"
		} else if userState == LIKED {
			state = "liked"
		}
		r.Relationships = append(r.Relationships, RelationShip{UserId: userId, State: state, Type: "relationship"})
	}
}

func checkErr(err error) {
	if err != nil {
		log.Fatalf("err : %v", err)
	}
}

func recovery() {
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()
}

func main() {
	dbTmp, err := sql.Open("postgres", "user=postgres password=123456 dbname=postgres sslmode=disable")
	checkErr(err)
	dbTmp.SetMaxOpenConns(60)
	dbTmp.SetMaxIdleConns(20)
	DB = dbTmp
	router := httprouter.New()
	router.GET("/users", getusers)
	router.POST("/users", adduser)
	router.GET("/users/:user_id/relationships", getUserRelationships)
	router.PUT("/users/:user_id/relationships/:other_user_id", addOrUpdateRelationships)

	log.Println(http.ListenAndServe(":9093", router))
}
